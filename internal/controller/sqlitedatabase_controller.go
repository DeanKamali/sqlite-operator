/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	databasev1alpha1 "github.com/sqlite-operator/sqlite-operator/api/v1alpha1"
)

// LitestreamConfig represents the Litestream configuration structure
type LitestreamConfig struct {
	DBs []LitestreamDB `yaml:"dbs"`
}

type LitestreamDB struct {
	Path    string            `yaml:"path"`
	Replica LitestreamReplica `yaml:"replica"`
}

type LitestreamReplica struct {
	URL                    string  `yaml:"url"`
	Region                 *string `yaml:"region,omitempty"`
	Retention              *string `yaml:"retention,omitempty"`
	RetentionCheckInterval *string `yaml:"retention-check-interval,omitempty"`
	Endpoint               *string `yaml:"endpoint,omitempty"`
}

// SqliteDatabaseReconciler reconciles a SqliteDatabase object
type SqliteDatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=database.sqlite.io,resources=sqlitedatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.sqlite.io,resources=sqlitedatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.sqlite.io,resources=sqlitedatabases/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SqliteDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the SqliteDatabase instance
	sqliteDB := &databasev1alpha1.SqliteDatabase{}
	if err := r.Get(ctx, req.NamespacedName, sqliteDB); err != nil {
		if errors.IsNotFound(err) {
			log.Info("SqliteDatabase resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get SqliteDatabase")
		return ctrl.Result{}, err
	}

	// Set default values
	r.setDefaults(sqliteDB)

	// Update status with observed generation
	if sqliteDB.Status.ObservedGeneration != sqliteDB.Generation {
		sqliteDB.Status.ObservedGeneration = sqliteDB.Generation
		if err := r.Status().Update(ctx, sqliteDB); err != nil {
			log.Error(err, "Failed to update observed generation")
			return ctrl.Result{}, err
		}
	}

	// Create/Update PVC
	if err := r.reconcilePVC(ctx, sqliteDB); err != nil {
		log.Error(err, "Failed to reconcile PVC")
		return ctrl.Result{}, err
	}

	// Create/Update Litestream ConfigMap if enabled
	if sqliteDB.Spec.Litestream != nil && sqliteDB.Spec.Litestream.Enabled {
		if err := r.reconcileLitestreamConfig(ctx, sqliteDB); err != nil {
			log.Error(err, "Failed to reconcile Litestream ConfigMap")
			return ctrl.Result{}, err
		}
	}

	// Create/Update sqlite-rest ConfigMap if enabled
	if sqliteDB.Spec.SqliteRest != nil && sqliteDB.Spec.SqliteRest.Enabled {
		if err := r.reconcileSqliteRestConfig(ctx, sqliteDB); err != nil {
			log.Error(err, "Failed to reconcile sqlite-rest ConfigMap")
			return ctrl.Result{}, err
		}
	}

	// Create/Update Deployment
	if err := r.reconcileDeployment(ctx, sqliteDB); err != nil {
		log.Error(err, "Failed to reconcile Deployment")
		return ctrl.Result{}, err
	}

	// Create/Update Service if sqlite-rest is enabled
	if sqliteDB.Spec.SqliteRest != nil && sqliteDB.Spec.SqliteRest.Enabled {
		if err := r.reconcileService(ctx, sqliteDB); err != nil {
			log.Error(err, "Failed to reconcile Service")
			return ctrl.Result{}, err
		}
	}

	// Create/Update Ingress if enabled
	if sqliteDB.Spec.Ingress != nil && sqliteDB.Spec.Ingress.Enabled {
		if err := r.reconcileIngress(ctx, sqliteDB); err != nil {
			log.Error(err, "Failed to reconcile Ingress")
			return ctrl.Result{}, err
		}
	}

	// Update status
	if err := r.updateStatus(ctx, sqliteDB); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SqliteDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.SqliteDatabase{}).
		Named("sqlitedatabase").
		Complete(r)
}

// setDefaults sets default values for the SqliteDatabase
func (r *SqliteDatabaseReconciler) setDefaults(sqliteDB *databasev1alpha1.SqliteDatabase) {
	// Set default database name if not specified
	if sqliteDB.Spec.Database.Name == "" {
		sqliteDB.Spec.Database.Name = "database.db"
	}

	// Set default storage size if not specified
	if sqliteDB.Spec.Database.Storage.Size == "" {
		sqliteDB.Spec.Database.Storage.Size = "1Gi"
	}

	// Set default Litestream enabled if not specified
	if sqliteDB.Spec.Litestream == nil {
		sqliteDB.Spec.Litestream = &databasev1alpha1.LitestreamConfig{
			Enabled: true,
		}
	}

	// Set default sqlite-rest disabled if not specified (sidecar mode)
	if sqliteDB.Spec.SqliteRest == nil {
		sqliteDB.Spec.SqliteRest = &databasev1alpha1.SqliteRestConfig{
			Enabled: false,
			Port:    8080,
			Metrics: &databasev1alpha1.MetricsConfig{
				Enabled: true,
				Port:    8081,
			},
		}
	}

	// Set default access mode to ReadWriteMany for sidecar mode
	if sqliteDB.Spec.Database.Storage.AccessMode == "" {
		sqliteDB.Spec.Database.Storage.AccessMode = "ReadWriteMany"
	}

	// Set default Ingress disabled if not specified
	if sqliteDB.Spec.Ingress == nil {
		sqliteDB.Spec.Ingress = &databasev1alpha1.IngressConfig{
			Enabled: false,
		}
	}
}

// reconcilePVC creates or updates the PersistentVolumeClaim
func (r *SqliteDatabaseReconciler) reconcilePVC(ctx context.Context, sqliteDB *databasev1alpha1.SqliteDatabase) error {
	// Convert string to access mode
	accessMode := corev1.ReadWriteOnce
	switch sqliteDB.Spec.Database.Storage.AccessMode {
	case "ReadWriteMany":
		accessMode = corev1.ReadWriteMany
	case "ReadOnlyMany":
		accessMode = corev1.ReadOnlyMany
	case "ReadWriteOnce":
		accessMode = corev1.ReadWriteOnce
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-db-storage", sqliteDB.Name),
			Namespace: sqliteDB.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "sqlite-database",
				"app.kubernetes.io/instance":   sqliteDB.Name,
				"app.kubernetes.io/managed-by": "sqlite-operator",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(sqliteDB.Spec.Database.Storage.Size),
				},
			},
		},
	}

	if sqliteDB.Spec.Database.Storage.StorageClass != nil {
		pvc.Spec.StorageClassName = sqliteDB.Spec.Database.Storage.StorageClass
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		// Set owner reference
		return controllerutil.SetControllerReference(sqliteDB, pvc, r.Scheme)
	})

	return err
}

// reconcileLitestreamConfig creates or updates the Litestream ConfigMap
func (r *SqliteDatabaseReconciler) reconcileLitestreamConfig(ctx context.Context, sqliteDB *databasev1alpha1.SqliteDatabase) error {
	config := r.buildLitestreamConfig(sqliteDB)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-litestream-config", sqliteDB.Name),
			Namespace: sqliteDB.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "sqlite-database",
				"app.kubernetes.io/instance":   sqliteDB.Name,
				"app.kubernetes.io/managed-by": "sqlite-operator",
			},
		},
		Data: map[string]string{
			"litestream.yml": config,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		return controllerutil.SetControllerReference(sqliteDB, configMap, r.Scheme)
	})

	return err
}

// buildLitestreamConfig generates the Litestream configuration YAML
func (r *SqliteDatabaseReconciler) buildLitestreamConfig(sqliteDB *databasev1alpha1.SqliteDatabase) string {
	var dbs []LitestreamDB

	for _, replica := range sqliteDB.Spec.Litestream.Replicas {
		url := r.buildReplicaURL(replica)

		litestreamReplica := LitestreamReplica{
			URL: url,
		}

		if replica.Region != nil {
			litestreamReplica.Region = replica.Region
		}
		if replica.Retention != nil {
			litestreamReplica.Retention = replica.Retention
		}
		if replica.RetentionCheckInterval != nil {
			litestreamReplica.RetentionCheckInterval = replica.RetentionCheckInterval
		}
		if replica.Endpoint != nil {
			litestreamReplica.Endpoint = replica.Endpoint
		}

		db := LitestreamDB{
			Path:    fmt.Sprintf("/var/lib/sqlite/%s", sqliteDB.Spec.Database.Name),
			Replica: litestreamReplica,
		}

		dbs = append(dbs, db)
	}

	config := LitestreamConfig{
		DBs: dbs,
	}

	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		// Fallback to simple string format if YAML marshaling fails
		return fmt.Sprintf("dbs:\n  - path: /var/lib/sqlite/%s\n    replica:\n      url: %s",
			sqliteDB.Spec.Database.Name,
			r.buildReplicaURL(sqliteDB.Spec.Litestream.Replicas[0]))
	}

	return string(yamlBytes)
}

// buildReplicaURL builds the URL for a replica based on its type
func (r *SqliteDatabaseReconciler) buildReplicaURL(replica databasev1alpha1.ReplicaConfig) string {
	path := ""
	if replica.Path != nil {
		path = *replica.Path
	}

	switch replica.Type {
	case "s3":
		return fmt.Sprintf("s3://%s/%s", replica.Bucket, path)
	case "azure":
		return fmt.Sprintf("abs://%s/%s", replica.Bucket, path)
	case "gcs":
		return fmt.Sprintf("gs://%s/%s", replica.Bucket, path)
	case "local":
		return fmt.Sprintf("file:///backups/%s", path)
	default:
		return fmt.Sprintf("s3://%s/%s", replica.Bucket, path)
	}
}

// reconcileSqliteRestConfig creates or updates the sqlite-rest ConfigMap
func (r *SqliteDatabaseReconciler) reconcileSqliteRestConfig(ctx context.Context, sqliteDB *databasev1alpha1.SqliteDatabase) error {
	config := r.buildSqliteRestConfig(sqliteDB)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sqlite-rest-config", sqliteDB.Name),
			Namespace: sqliteDB.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "sqlite-database",
				"app.kubernetes.io/instance":   sqliteDB.Name,
				"app.kubernetes.io/managed-by": "sqlite-operator",
			},
		},
		Data: map[string]string{
			"sqlite-rest.yml": config,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		return controllerutil.SetControllerReference(sqliteDB, configMap, r.Scheme)
	})

	return err
}

// buildSqliteRestConfig generates the sqlite-rest configuration YAML
func (r *SqliteDatabaseReconciler) buildSqliteRestConfig(sqliteDB *databasev1alpha1.SqliteDatabase) string {
	config := fmt.Sprintf(`server:
  addr: ":%d"
  database:
    dsn: "/var/lib/sqlite/%s"`, sqliteDB.Spec.SqliteRest.Port, sqliteDB.Spec.Database.Name)

	if sqliteDB.Spec.SqliteRest.AuthSecret != nil {
		config += "\n  auth-token-file: \"/etc/auth/token\""
	}

	if len(sqliteDB.Spec.SqliteRest.AllowedTables) > 0 {
		config += fmt.Sprintf("\n  security-allow-table: \"%s\"", strings.Join(sqliteDB.Spec.SqliteRest.AllowedTables, ","))
	}

	if sqliteDB.Spec.SqliteRest.Metrics != nil && sqliteDB.Spec.SqliteRest.Metrics.Enabled {
		config += fmt.Sprintf("\n  metrics-addr: \":%d\"", sqliteDB.Spec.SqliteRest.Metrics.Port)
	}

	return config
}

// reconcileDeployment creates or updates the Deployment
func (r *SqliteDatabaseReconciler) reconcileDeployment(ctx context.Context, sqliteDB *databasev1alpha1.SqliteDatabase) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sqliteDB.Name,
			Namespace: sqliteDB.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "sqlite-database",
				"app.kubernetes.io/instance":   sqliteDB.Name,
				"app.kubernetes.io/managed-by": "sqlite-operator",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "sqlite-database",
					"app.kubernetes.io/instance": sqliteDB.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "sqlite-database",
						"app.kubernetes.io/instance": sqliteDB.Name,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: r.buildInitContainers(sqliteDB),
					Containers:     r.buildContainers(sqliteDB),
					Volumes:        r.buildVolumes(sqliteDB),
				},
			},
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		return controllerutil.SetControllerReference(sqliteDB, deployment, r.Scheme)
	})

	return err
}

// buildInitContainers builds the init container specifications
func (r *SqliteDatabaseReconciler) buildInitContainers(sqliteDB *databasev1alpha1.SqliteDatabase) []corev1.Container {
	initContainers := []corev1.Container{
		{
			Name:    "init-db",
			Image:   "keinos/sqlite3:latest",
			Command: []string{"/bin/sh", "-c"},
			Args: []string{fmt.Sprintf(`
				set -e
				mkdir -p /var/lib/sqlite
				if [ ! -f /var/lib/sqlite/%s ]; then
					echo "Creating empty database..."
					sqlite3 /var/lib/sqlite/%s "SELECT 1;"
					echo "Database created at /var/lib/sqlite/%s"
				else
					echo "Database already exists"
				fi`, sqliteDB.Spec.Database.Name, sqliteDB.Spec.Database.Name, sqliteDB.Spec.Database.Name)},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "db-storage",
					MountPath: "/var/lib/sqlite",
				},
			},
		},
	}

	// Optionally add init script volume mount if configured
	if sqliteDB.Spec.Database.InitScript != nil {
		initContainers[0].VolumeMounts = append(initContainers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "init-script",
			MountPath: "/init",
		})
		initContainers[0].Args[0] = r.buildSqliteInitScript(sqliteDB)
	}

	return initContainers
}

// buildContainers builds the container specifications
func (r *SqliteDatabaseReconciler) buildContainers(sqliteDB *databasev1alpha1.SqliteDatabase) []corev1.Container {
	containers := []corev1.Container{}

	// Note: SQLite is now handled by init container for sidecar mode

	// Litestream container if enabled
	if sqliteDB.Spec.Litestream != nil && sqliteDB.Spec.Litestream.Enabled {
		litestreamContainer := corev1.Container{
			Name:    "litestream",
			Image:   "litestream/litestream:latest",
			Command: []string{"litestream"},
			Args:    []string{"replicate", "-config", "/etc/litestream/litestream.yml"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "db-storage",
					MountPath: "/var/lib/sqlite",
				},
				{
					Name:      "litestream-config",
					MountPath: "/etc/litestream",
				},
			},
		}

		// Add environment variables for credentials
		for _, replica := range sqliteDB.Spec.Litestream.Replicas {
			if replica.Credentials != nil {
				litestreamContainer.Env = append(litestreamContainer.Env, []corev1.EnvVar{
					{
						Name: "LITESTREAM_ACCESS_KEY_ID",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: replica.Credentials.SecretName,
								},
								Key: getStringValue(replica.Credentials.AccessKeyField, "access-key"),
							},
						},
					},
					{
						Name: "LITESTREAM_SECRET_ACCESS_KEY",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: replica.Credentials.SecretName,
								},
								Key: getStringValue(replica.Credentials.SecretKeyField, "secret-key"),
							},
						},
					},
				}...)
			}
		}

		containers = append(containers, litestreamContainer)
	}

	// sqlite-rest container if enabled
	if sqliteDB.Spec.SqliteRest != nil && sqliteDB.Spec.SqliteRest.Enabled {
		sqliteRestContainer := corev1.Container{
			Name:  "sqlite-rest",
			Image: "ghcr.io/b4fun/sqlite-rest/server:main",
			Args:  r.buildSqliteRestArgs(sqliteDB),
			Ports: r.buildSqliteRestPorts(sqliteDB),
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "db-storage",
					MountPath: "/var/lib/sqlite",
				},
			},
		}

		containers = append(containers, sqliteRestContainer)
	}

	return containers
}

// buildVolumes builds the volume specifications
func (r *SqliteDatabaseReconciler) buildVolumes(sqliteDB *databasev1alpha1.SqliteDatabase) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "db-storage",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-db-storage", sqliteDB.Name),
				},
			},
		},
	}

	// Add init script volume if specified
	if sqliteDB.Spec.Database.InitScript != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "init-script",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *sqliteDB.Spec.Database.InitScript,
					},
				},
			},
		})
	}

	// Add Litestream volumes if enabled
	if sqliteDB.Spec.Litestream != nil && sqliteDB.Spec.Litestream.Enabled {
		volumes = append(volumes, []corev1.Volume{
			{
				Name: "litestream-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: fmt.Sprintf("%s-litestream-config", sqliteDB.Name),
						},
					},
				},
			},
		}...)
	}

	// Add sqlite-rest volumes if enabled
	if sqliteDB.Spec.SqliteRest != nil && sqliteDB.Spec.SqliteRest.Enabled {
		volumes = append(volumes, []corev1.Volume{
			{
				Name: "sqlite-rest-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: fmt.Sprintf("%s-sqlite-rest-config", sqliteDB.Name),
						},
					},
				},
			},
		}...)

		// Add auth secret volume if specified
		if sqliteDB.Spec.SqliteRest.AuthSecret != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "sqlite-rest-auth",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: *sqliteDB.Spec.SqliteRest.AuthSecret,
					},
				},
			})
		}
	}

	return volumes
}

// buildSqliteInitScript generates the SQLite initialization script
func (r *SqliteDatabaseReconciler) buildSqliteInitScript(sqliteDB *databasev1alpha1.SqliteDatabase) string {
	script := `set -e
mkdir -p /var/lib/sqlite`

	if sqliteDB.Spec.Database.InitScript != nil {
		script += fmt.Sprintf(`
if [ ! -f /var/lib/sqlite/%s ]; then
  echo "Initializing database with init script..."
  sqlite3 /var/lib/sqlite/%s < /init/init.sql
fi`, sqliteDB.Spec.Database.Name, sqliteDB.Spec.Database.Name)
	} else {
		script += fmt.Sprintf(`
# Create empty database if no init script
if [ ! -f /var/lib/sqlite/%s ]; then
  echo "Creating empty database..."
  sqlite3 /var/lib/sqlite/%s "SELECT 1;"
fi`, sqliteDB.Spec.Database.Name, sqliteDB.Spec.Database.Name)
	}

	script += fmt.Sprintf(`
echo "Database ready at /var/lib/sqlite/%s"
tail -f /dev/null`, sqliteDB.Spec.Database.Name)

	return script
}

// buildSqliteRestArgs builds the sqlite-rest container arguments
func (r *SqliteDatabaseReconciler) buildSqliteRestArgs(sqliteDB *databasev1alpha1.SqliteDatabase) []string {
	args := []string{
		"serve",
		"--db-dsn", fmt.Sprintf("/var/lib/sqlite/%s", sqliteDB.Spec.Database.Name),
		"--http-addr", fmt.Sprintf(":%d", sqliteDB.Spec.SqliteRest.Port),
	}

	if sqliteDB.Spec.SqliteRest.Metrics != nil && sqliteDB.Spec.SqliteRest.Metrics.Enabled {
		args = append(args, "--metrics-addr", fmt.Sprintf(":%d", sqliteDB.Spec.SqliteRest.Metrics.Port))
	}

	for _, table := range sqliteDB.Spec.SqliteRest.AllowedTables {
		args = append(args, "--security-allow-table", table)
	}

	if sqliteDB.Spec.SqliteRest.AuthSecret != nil {
		args = append(args, "--auth-token-file", "/etc/auth/token")
	}
	// Note: sqlite-rest does not have a --no-auth flag
	// If no auth is configured, the server will run without authentication

	return args
}

// buildSqliteRestPorts builds the sqlite-rest container ports
func (r *SqliteDatabaseReconciler) buildSqliteRestPorts(sqliteDB *databasev1alpha1.SqliteDatabase) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          "http",
			ContainerPort: sqliteDB.Spec.SqliteRest.Port,
		},
	}

	if sqliteDB.Spec.SqliteRest.Metrics != nil && sqliteDB.Spec.SqliteRest.Metrics.Enabled {
		ports = append(ports, corev1.ContainerPort{
			Name:          "metrics",
			ContainerPort: sqliteDB.Spec.SqliteRest.Metrics.Port,
		})
	}

	return ports
}

// reconcileService creates or updates the Service
func (r *SqliteDatabaseReconciler) reconcileService(ctx context.Context, sqliteDB *databasev1alpha1.SqliteDatabase) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-service", sqliteDB.Name),
			Namespace: sqliteDB.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "sqlite-database",
				"app.kubernetes.io/instance":   sqliteDB.Name,
				"app.kubernetes.io/managed-by": "sqlite-operator",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name":     "sqlite-database",
				"app.kubernetes.io/instance": sqliteDB.Name,
			},
			Ports: r.buildServicePorts(sqliteDB),
			Type:  corev1.ServiceTypeClusterIP,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		return controllerutil.SetControllerReference(sqliteDB, service, r.Scheme)
	})

	return err
}

// buildServicePorts builds the service ports
func (r *SqliteDatabaseReconciler) buildServicePorts(sqliteDB *databasev1alpha1.SqliteDatabase) []corev1.ServicePort {
	ports := []corev1.ServicePort{
		{
			Name:       "http",
			Port:       8080,
			TargetPort: intstr.FromInt(int(sqliteDB.Spec.SqliteRest.Port)),
		},
	}

	if sqliteDB.Spec.SqliteRest.Metrics != nil && sqliteDB.Spec.SqliteRest.Metrics.Enabled {
		ports = append(ports, corev1.ServicePort{
			Name:       "metrics",
			Port:       8081,
			TargetPort: intstr.FromInt(int(sqliteDB.Spec.SqliteRest.Metrics.Port)),
		})
	}

	return ports
}

// reconcileIngress creates or updates the Ingress
func (r *SqliteDatabaseReconciler) reconcileIngress(ctx context.Context, sqliteDB *databasev1alpha1.SqliteDatabase) error {
	if sqliteDB.Spec.Ingress.Host == nil {
		return fmt.Errorf("ingress host is required when ingress is enabled")
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ingress", sqliteDB.Name),
			Namespace: sqliteDB.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "sqlite-database",
				"app.kubernetes.io/instance":   sqliteDB.Name,
				"app.kubernetes.io/managed-by": "sqlite-operator",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: *sqliteDB.Spec.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &[]networkingv1.PathType{networkingv1.PathTypePrefix}[0],
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: fmt.Sprintf("%s-service", sqliteDB.Name),
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Add TLS configuration if enabled
	if sqliteDB.Spec.Ingress.TLS != nil && sqliteDB.Spec.Ingress.TLS.Enabled && sqliteDB.Spec.Ingress.TLS.SecretName != nil {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{*sqliteDB.Spec.Ingress.Host},
				SecretName: *sqliteDB.Spec.Ingress.TLS.SecretName,
			},
		}

		// Add cert-manager annotation
		if ingress.Annotations == nil {
			ingress.Annotations = make(map[string]string)
		}
		ingress.Annotations["cert-manager.io/cluster-issuer"] = "letsencrypt-prod"
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ingress, func() error {
		return controllerutil.SetControllerReference(sqliteDB, ingress, r.Scheme)
	})

	return err
}

// updateStatus updates the status of the SqliteDatabase
func (r *SqliteDatabaseReconciler) updateStatus(ctx context.Context, sqliteDB *databasev1alpha1.SqliteDatabase) error {
	// Check deployment status
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      sqliteDB.Name,
		Namespace: sqliteDB.Namespace,
	}, deployment)

	if err != nil {
		if errors.IsNotFound(err) {
			sqliteDB.Status.Phase = "Pending"
			sqliteDB.Status.Message = "Deployment not found"
		} else {
			sqliteDB.Status.Phase = "Failed"
			sqliteDB.Status.Message = fmt.Sprintf("Failed to get deployment: %v", err)
		}
	} else {
		if deployment.Status.ReadyReplicas > 0 {
			sqliteDB.Status.Phase = "Running"
			sqliteDB.Status.Message = "Database is running successfully"
			sqliteDB.Status.Replicas = deployment.Status.ReadyReplicas

			// Update endpoints
			if sqliteDB.Spec.SqliteRest != nil && sqliteDB.Spec.SqliteRest.Enabled {
				restURL := fmt.Sprintf("http://%s-service.%s.svc.cluster.local:%d",
					sqliteDB.Name, sqliteDB.Namespace, sqliteDB.Spec.SqliteRest.Port)
				sqliteDB.Status.Endpoints = &databasev1alpha1.EndpointsStatus{
					Rest: &restURL,
				}

				if sqliteDB.Spec.SqliteRest.Metrics != nil && sqliteDB.Spec.SqliteRest.Metrics.Enabled {
					metricsURL := fmt.Sprintf("http://%s-service.%s.svc.cluster.local:%d",
						sqliteDB.Name, sqliteDB.Namespace, sqliteDB.Spec.SqliteRest.Metrics.Port)
					sqliteDB.Status.Endpoints.Metrics = &metricsURL
				}
			}
		} else {
			sqliteDB.Status.Phase = "Pending"
			sqliteDB.Status.Message = "Deployment is starting"
		}
	}

	// Update conditions
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "ReconciliationSucceeded",
		Message:            sqliteDB.Status.Message,
	}

	if sqliteDB.Status.Phase != "Running" {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "ReconciliationInProgress"
	}

	// Update or add condition
	conditionIndex := -1
	for i, c := range sqliteDB.Status.Conditions {
		if c.Type == condition.Type {
			conditionIndex = i
			break
		}
	}

	if conditionIndex >= 0 {
		sqliteDB.Status.Conditions[conditionIndex] = condition
	} else {
		sqliteDB.Status.Conditions = append(sqliteDB.Status.Conditions, condition)
	}

	return r.Status().Update(ctx, sqliteDB)
}

// Helper functions
func int32Ptr(i int32) *int32 { return &i }

func getStringValue(ptr *string, defaultValue string) string {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}
