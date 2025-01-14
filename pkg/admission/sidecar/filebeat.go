package sidecar

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"

	"corp.wz.net/opsdev/log-collection/pkg/common"
	"corp.wz.net/opsdev/log-collection/pkg/filebeat"
)

func GetLogAndVolPath(srcPath string, collectorType common.LogCollectorType, subPath string) (string, string) {
	//如果srcPath是目录的格式，直接挂载当前目录
	volPath, logPath := filepath.Dir(srcPath), filepath.Base(srcPath)
	if strings.HasSuffix(srcPath, "/") {
		logPath = ""
	}

	if collectorType == common.SidecarMode {
		logPath = fmt.Sprintf(common.LscSidecarLogDir+"/%s/%s", subPath, logPath)
	} else {
		logPath = fmt.Sprintf(common.LscDaemonSetLogDir+"/%s/%s", subPath, logPath)
	}

	return logPath, volPath
}

func addLogVolumeMount(pod *corev1.Pod) error {
	lscConfig, err := filebeat.DecodeLogConfig(pod.Annotations[common.LscAnnotationName])
	if err != nil {
		return err
	}

	for cName, volumeLogConfig := range lscConfig.ContainerLogConfigs {
		if cName = strings.TrimSpace(cName); cName == "" {
			continue
		}
		queryOrderSpecInfo := filebeat.CalculateHowToMount(volumeLogConfig)
		for vName, pathConfig := range volumeLogConfig {
			if vName = strings.TrimSpace(vName); vName == "" {
				continue
			}
			for _, logRelPath := range pathConfig.Paths {
				if logRelPath = strings.TrimSpace(logRelPath); logRelPath == "" {
					continue
				}

				//这里对类型进行区分，如果存在匹配类型,则需要挂载*的前一级目录；如果没有，则直接挂载最后一级目录
				subPath := filebeat.CalSubPath(queryOrderSpecInfo, logRelPath, cName)
				_, volPath := GetLogAndVolPath(logRelPath, pathConfig.LogCollectorType, subPath)
				mount(pod, volPath, subPath, cName)
			}
		}
	}

	return err
}

func mount(pod *corev1.Pod, volPath, subPath string, cName string) {
	logVolMount := corev1.VolumeMount{
		Name:      "log-volume",
		MountPath: volPath,
		SubPath:   subPath,
	}
	//计算出来每个container需要挂载的路径，然后先直接挂载出来
	for icontainer, containName := range pod.Spec.Containers {
		if alreadyMount(&containName, volPath) {
			continue
		}
		if containName.Name == cName {
			pod.Spec.Containers[icontainer].VolumeMounts = append(pod.Spec.Containers[icontainer].VolumeMounts, logVolMount)
		}
	}
}

func alreadyMount(c *corev1.Container, mountPath string) bool {
	for _, mount := range c.VolumeMounts {
		if mount.MountPath == mountPath {
			return true
		}
	}

	return false
}

func getInputsData(inputs []common.InputsData) (string, error) {
	tplFuncMap := template.FuncMap{"Base": filebeat.Base}
	fbInputsYamlTpl, _ := template.New("inputs.yml.template").Funcs(tplFuncMap).ParseFiles(common.FilebeatInputsConfigTplPath)
	buffer := bytes.NewBufferString("")
	if err := fbInputsYamlTpl.Execute(buffer, &inputs); err != nil {
		return "", errors.New("function name:getInputsData do fbInputsYamlTpl.Execute error." + err.Error())
	}

	return buffer.String(), nil
}
