package common

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	LscAnnotationName = "logging.io/logsidecar-config"
	LscContainerName  = "logsidecar-filebeat"
	LscVolumeName     = "logsidecar-config-volume-logging-io"

	LscInitContainerName = "logsidecar-init"
	LscInitImage         = "r.soft.cn/cloud/envsubst:v0.1"

	LscAnnotationNeeded       = "logsidecar-inject.logging-enable"
	LscAnnotationRescource    = "logsidecar-inject.resource.config"
	LogAnnotationNeededEnable = "enable"

	LscSidecarLogDir   = "/data/sidecar-log"
	LscDaemonSetLogDir = "/data/controller-log"

	FilebeatConfTplDir          = "/opt/templates"
	FilebeatConfigTplPath       = "/opt/templates/filebeat.yml"
	FilebeatInputsConfigTplPath = "/opt/templates/inputs.yml.template"

	FilebeatConfDir               = "/etc/filebeat"
	FinalFilebeatConfigDir        = "/etc/filebeat/config"
	FinalFilebeatInputsConfigDir  = "/etc/filebeat/inputs.d"
	FinalFilebeatConfigPath       = "/etc/filebeat/config/filebeat.yml"
	FinalFilebeatInputsConfigPath = "/etc/filebeat/inputs.d/inputs.yml"

	CertDir = "/etc/logsidecar"
)

var LogSidecarImage = "harbor.wz.net/cloud/filebeat:492c95d7"

type LogCollectorType int

const (
	SidecarMode   LogCollectorType = 0
	DaemonsetMode LogCollectorType = 1
)

type LogType int

const (
	StdoutMode LogType = 0
	FileMode   LogType = 1
)

const (
	LogFormat   = "format"
	LogWZFormat = "wzFormat"
	LogJson     = "json"
)

type FilebeatInputConfigs struct {
	FBInputs []FilebeatInputsData
	Resource ResourceRequirements
}

type FilebeatInputsData struct {
	Hosts            string          `json:"hosts"`
	Paths            []string        `json:"paths"`
	HostsTopic       string          `json:"hosts_topic"`
	Topic            string          `json:"topic"`
	MultilineEnable  bool            `json:"multiline_enable"`
	MultilinePattern MultilineConfig `json:"multiline_pattern"`
	CustomField      string          `json:"custom_field"`
	Codec            string          `json:"codec"`
	Prefix           string          `json:"prefix"`
}

type FilebeatYmlConfigData struct {
	Kafka KafkaOutput
	ES    ESOutput
}

type KafkaOutput struct {
	Hosts string
	Topic string
}

type ESOutput struct {
	Hosts string
}

type FilebeatOutputData struct {
	Kafka KafkaOutput
	ES    ESOutput
}

type InputsData struct {
	ContainerLogConfigs ContainerLogConfigs `json:"containerLogConfigs,omitempty"`
	CustomField         string              `json:"custom_field"`
	Prefix              string              `json:"prefix"`
}

type LSCConfig struct {
	ContainerLogConfigs ContainerLogConfigs  `json:"containerLogConfigs,omitempty"`
	SidecarResources    ResourceRequirements `json:"sidecar_resources"`
}

type ResourceRequirements struct {
	Limits   map[corev1.ResourceName]string `json:"limits"`
	Requests map[corev1.ResourceName]string `json:"requests"`
}

type ContainerLogConfigs map[string]VolumeLogConfig // key: containerName; value: VolumeLogConfig
type VolumeLogConfig map[string]VolumePathConfig    // key: volumeName; value: Config
type VolumePathConfig struct {
	LogCollectorType LogCollectorType `json:"log_collector_type"` //0: controller, 1: controller
	Paths            []string         `json:"paths,omitempty"`
	Topic            string           `json:"topic,omitempty"`
	Hosts            string           `json:"hosts"`
	MultilineEnable  bool             `json:"multiline_enable"`
	MultilinePattern MultilineConfig  `json:"multiline_pattern,omitempty"`
	LogType          LogType          `json:"log_type,omitempty"` //0: stdout, 1: file
	Codec            string           `json:"codec"`              // "json", "format", "wzFormat"
	Prefix           string           `json:"prefix"`
	Cluster          string           `json:"cluster"`
}

type MultilineConfig struct {
	MulPattern string `json:"multiline_pattern"`
	MulNegate  string `json:"multiline_negate"`
	MulMatch   string `json:"multiline_match"`
}

type LogConfigVolumePath struct {
	ContainName string
	VolumePath  string
	LogPath     string
}
