package sidecar

import (
	"corp.wz.net/opsdev/log-collection/pkg/common"
	"corp.wz.net/opsdev/log-collection/pkg/filebeat"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"os"
)

var (
	sidecarCPULimit      = "200m"
	sidecarMemoryLimit   = "200Mi"
	sidecarCPURequest    = "100m"
	sidecarMemoryRequest = "100Mi"
)

//大致步骤：
//1. init container挂载 logsidecar-helper生成的filebeat配置,替换运行时环境变量,执行命令：envsubst < decoder.conf.template > decoder.conf
//2. logsidecar 和 init container通过emptydir 挂载filebeat的配置目录
//3. logsidecar 通过挂载emptydir的volume读取业务容器的日志
func (i *Injector) ensurePod(pod *corev1.Pod, appName string) {
	lscConfig, err := filebeat.DecodeLogConfig(pod.Annotations[common.LscAnnotationName])
	if err != nil {
		return
	}

	//走到这里，说明是需要收集日志的，不管daemonset还是sidecar模式都需要挂载log目录
	v := corev1.Volume{
		Name: "log-volume",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	addVolume(&pod.Spec.Volumes, v)
	addLogVolumeMount(pod)

	//如果不是sidecar模式，不需要往下走了
	if !filebeat.IsCollectLog(*lscConfig, common.SidecarMode) {
		return
	}

	//添加所有要用的volume,主要包括：
	//1.helper生成的configmap(filebeat的配置)，
	//2.渲染env后的配置，需要通过一个emptydir的vlome同步给sidecar
	//3.业务的日志也需要emptydir同步给sidecar
	ensureVolume(pod, appName)
	//生成替换helper生成的filebeat配置文件中的环境变量为运行时值的容器
	container := generateEnvSubStContainer()
	//添加到pod的init container中
	addContainer(&pod.Spec.InitContainers, container)

	//生成sidecar容器
	container = generateFilebeatContainerByResource(lscConfig.SidecarResources)
	//添加到pod的container中
	addContainer(&pod.Spec.Containers, container)
}

func addVolume(volumes *[]corev1.Volume, volume corev1.Volume) {
	//删除原本同名的volume
	for i, ic := range *volumes {
		if ic.Name == volume.Name {
			*volumes = append((*volumes)[:i], (*volumes)[i+1:]...)
		}
	}

	*volumes = append(*volumes, volume)
}

func ensureVolume(pod *corev1.Pod, appName string) {
	//添加新的volume
	v := corev1.Volume{
		Name: common.LscVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	addVolume(&pod.Spec.Volumes, v)

	v = corev1.Volume{
		Name: "origin-filebeat-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: appName + "-log-sidecar-cm",
				},
			},
		},
	}
	addVolume(&pod.Spec.Volumes, v)
}

func generateEnvSubStContainer() corev1.Container {
	//替换filebeat 配置中的环境变量为容器运行时的值
	cmd := fmt.Sprintf(`mkdir -p %s && 
		export IP=$(ifconfig eth0 | grep "inet addr:" | awk '{print $2}' | cut -c 6-) && 
		envsubst <%s> %s`,
		common.FinalFilebeatInputsConfigDir,
		common.FilebeatInputsConfigTplPath,
		common.FinalFilebeatInputsConfigPath)

	return corev1.Container{
		Name:            common.LscInitContainerName,
		Image:           common.LscInitImage,
		ImagePullPolicy: corev1.PullAlways,
		Command:         []string{"/bin/sh"},
		Args:            []string{"-c", cmd},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      common.LscVolumeName,
				MountPath: common.FinalFilebeatInputsConfigDir,
			},
			{
				Name:      "origin-filebeat-config",
				MountPath: common.FilebeatConfTplDir,
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(sidecarCPULimit),
				corev1.ResourceMemory: resource.MustParse(sidecarMemoryLimit),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(sidecarCPURequest),
				corev1.ResourceMemory: resource.MustParse(sidecarMemoryRequest),
			},
		},
	}
}

func generateFilebeatContainerByResource(resources common.ResourceRequirements) corev1.Container {
	return corev1.Container{
		Name:            common.LscContainerName,
		Image:           common.LogSidecarImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"-c",
			common.FinalFilebeatConfigPath,
			"--path.config",
			"/etc/filebeat",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "IDC",
				Value: os.Getenv("IDC"),
			},
			{
				Name: "NODENAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "spec.nodeName",
					},
				},
			},
		},
		Resources: ensureRequireMent(resources),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      common.LscVolumeName,
				MountPath: common.FinalFilebeatInputsConfigDir,
			},
			{
				Name:      "log-volume",
				MountPath: common.LscSidecarLogDir,
			},
			{
				Name:      "origin-filebeat-config",
				MountPath: common.FinalFilebeatConfigPath,
				SubPath:   "filebeat.yml",
			},
		},
		//Lifecycle: &corev1.Lifecycle{
		//	PreStop: &corev1.Handler{
		//		Exec: &corev1.ExecAction{
		//			Command: []string{"/usr/local/bin/preStop.sh"},
		//		},
		//	},
		//},
	}
}

func addContainer(containers *[]corev1.Container, container corev1.Container) {
	//删除原本容器中和sidecar container同名的容器
	for i, ic := range *containers {
		if ic.Name == container.Name {
			*containers = append((*containers)[:i], (*containers)[i+1:]...)
		}
	}

	*containers = append(*containers, container)
}

func ensureRequireMent(res common.ResourceRequirements) corev1.ResourceRequirements {
	r := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(sidecarCPULimit),
			corev1.ResourceMemory: resource.MustParse(sidecarMemoryLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(sidecarCPURequest),
			corev1.ResourceMemory: resource.MustParse(sidecarMemoryRequest),
		},
	}

	if val, ok := res.Limits[corev1.ResourceMemory]; ok {
		r.Limits[corev1.ResourceMemory] = resource.MustParse(val)
	}
	if val, ok := res.Limits[corev1.ResourceCPU]; ok {
		r.Limits[corev1.ResourceCPU] = resource.MustParse(val)
	}
	if val, ok := res.Requests[corev1.ResourceMemory]; ok {
		r.Requests[corev1.ResourceMemory] = resource.MustParse(val)
	}
	if val, ok := res.Requests[corev1.ResourceCPU]; ok {
		r.Requests[corev1.ResourceCPU] = resource.MustParse(val)
	}

	return r
}
