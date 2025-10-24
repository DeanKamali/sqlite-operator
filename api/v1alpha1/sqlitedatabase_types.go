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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SqliteDatabaseSpec defines the desired state of SqliteDatabase.
type SqliteDatabaseSpec struct {
	// Database configuration
	Database DatabaseConfig `json:"database,omitempty"`

	// Litestream replication configuration
	Litestream *LitestreamConfig `json:"litestream,omitempty"`

	// SQLite REST API configuration
	SqliteRest *SqliteRestConfig `json:"sqliteRest,omitempty"`

	// Ingress configuration for external access
	Ingress *IngressConfig `json:"ingress,omitempty"`

	// Resource requirements for the pod
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// DatabaseConfig defines SQLite database configuration
type DatabaseConfig struct {
	// Name of the SQLite database file
	// +kubebuilder:default="database.db"
	Name string `json:"name"`

	// Name of ConfigMap containing SQL initialization script
	InitScript *string `json:"initScript,omitempty"`

	// Storage configuration for the database
	Storage StorageConfig `json:"storage"`
}

// StorageConfig defines storage configuration for the database
type StorageConfig struct {
	// Size of the persistent volume
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(E|P|T|G|M|K|Ei|Pi|Ti|Gi|Mi|Ki)?)$"
	Size string `json:"size"`

	// Storage class for the persistent volume
	StorageClass *string `json:"storageClass,omitempty"`

	// Access mode for the persistent volume
	// +kubebuilder:default="ReadWriteMany"
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadWriteMany;ReadOnlyMany
	AccessMode string `json:"accessMode,omitempty"`
}

// LitestreamConfig defines Litestream replication configuration
type LitestreamConfig struct {
	// Enable Litestream replication
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// List of replication targets
	Replicas []ReplicaConfig `json:"replicas,omitempty"`
}

// ReplicaConfig defines individual replica configuration
type ReplicaConfig struct {
	// Type of storage backend
	// +kubebuilder:validation:Enum=s3;azure;gcs;local
	Type string `json:"type"`

	// Bucket name for S3/GCS or container name for Azure
	Bucket string `json:"bucket"`

	// Region for S3/GCS
	Region *string `json:"region,omitempty"`

	// Custom S3 endpoint (e.g., wasabisys.com for Wasabi)
	Endpoint *string `json:"endpoint,omitempty"`

	// Path within the bucket/container
	Path *string `json:"path,omitempty"`

	// Credentials for the storage backend
	Credentials *CredentialsConfig `json:"credentials,omitempty"`

	// Retention period for backups
	// +kubebuilder:default="24h"
	Retention *string `json:"retention,omitempty"`

	// How often to check for expired backups
	// +kubebuilder:default="1h"
	RetentionCheckInterval *string `json:"retentionCheckInterval,omitempty"`
}

// CredentialsConfig defines credentials for storage backends
type CredentialsConfig struct {
	// Name of the Secret containing credentials
	SecretName string `json:"secretName"`

	// Field name for access key in the secret
	// +kubebuilder:default="access-key"
	AccessKeyField *string `json:"accessKeyField,omitempty"`

	// Field name for secret key in the secret
	// +kubebuilder:default="secret-key"
	SecretKeyField *string `json:"secretKeyField,omitempty"`
}

// SqliteRestConfig defines sqlite-rest API configuration
type SqliteRestConfig struct {
	// Enable sqlite-rest API
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Port for sqlite-rest API
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Name of Secret containing JWT token/key
	AuthSecret *string `json:"authSecret,omitempty"`

	// List of tables allowed for API access
	AllowedTables []string `json:"allowedTables,omitempty"`

	// Metrics configuration
	Metrics *MetricsConfig `json:"metrics,omitempty"`
}

// MetricsConfig defines metrics configuration
type MetricsConfig struct {
	// Enable metrics
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Port for metrics
	// +kubebuilder:default=8081
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}

// IngressConfig defines ingress configuration for external access
type IngressConfig struct {
	// Enable Ingress
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Hostname for the Ingress
	Host *string `json:"host,omitempty"`

	// TLS configuration
	TLS *TLSConfig `json:"tls,omitempty"`
}

// TLSConfig defines TLS configuration
type TLSConfig struct {
	// Enable TLS
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Name of TLS secret
	SecretName *string `json:"secretName,omitempty"`
}

// SqliteDatabaseStatus defines the observed state of SqliteDatabase.
type SqliteDatabaseStatus struct {
	// Current phase of the database
	// +kubebuilder:validation:Enum=Pending;Running;Failed;Terminating
	Phase string `json:"phase,omitempty"`

	// Human-readable message about the current status
	Message string `json:"message,omitempty"`

	// Number of active replicas
	Replicas int32 `json:"replicas,omitempty"`

	// Timestamp of the last successful backup
	LastBackup *metav1.Time `json:"lastBackup,omitempty"`

	// API endpoints information
	Endpoints *EndpointsStatus `json:"endpoints,omitempty"`

	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// EndpointsStatus defines API endpoints information
type EndpointsStatus struct {
	// REST API endpoint URL
	Rest *string `json:"rest,omitempty"`

	// Metrics endpoint URL
	Metrics *string `json:"metrics,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SqliteDatabase is the Schema for the sqlitedatabases API.
type SqliteDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SqliteDatabaseSpec   `json:"spec,omitempty"`
	Status SqliteDatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SqliteDatabaseList contains a list of SqliteDatabase.
type SqliteDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SqliteDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SqliteDatabase{}, &SqliteDatabaseList{})
}
