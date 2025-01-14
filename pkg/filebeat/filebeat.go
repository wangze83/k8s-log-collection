package filebeat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"corp.wz.net/opsdev/log-collection/pkg/common"
)

func IsCollectLog(logconfig common.LSCConfig, mode common.LogCollectorType) bool {
	need := false

	for cName, containerConf := range logconfig.ContainerLogConfigs {
		if strings.TrimSpace(cName) == "" {
			continue
		}
		for vName, volumeConf := range containerConf {
			if strings.TrimSpace(vName) == "" {
				continue
			}
			if volumeConf.LogCollectorType == mode {
				need = true
			}
		}
	}

	return need
}

func getInputsData(inputs []common.InputsData) *common.FilebeatInputConfigs {
	var srcInputs []common.FilebeatInputsData

	for _, input := range inputs {
		for cName, volumeConfig := range input.ContainerLogConfigs {
			if cName == "" {
				continue
			}
			queryOrderSpecInfo := CalculateHowToMount(volumeConfig)
			for vName, pathConfig := range volumeConfig {
				if vName == "" ||
					pathConfig.LogType != common.FileMode ||
					pathConfig.LogCollectorType != common.SidecarMode {
					continue
				}
				paths := make([]string, 0)
				for _, logRelPath := range pathConfig.Paths {
					if logRelPath == "" {
						continue
					}
					subPath := CalSubPath(queryOrderSpecInfo, logRelPath, cName)
					p := fmt.Sprintf("%s/%s/%s", common.LscSidecarLogDir, subPath, Base(logRelPath))
					paths = append(paths, p)
				}

				// pathConfig.Codec == "wzFormat"时，日志格式为原wz模式
				// 匹配原Kafka日志收集格式前缀: [idc=**,app=**,pod=**,filename=**]
				prefix := input.Prefix
				if pathConfig.Codec == common.LogWZFormat {
					// pathConfig.Paths为多个日志文件时，不能确定filename具体名称
					if len(pathConfig.Paths) == 1 {
						prefix = "\"[" + prefix + ",filename=" + Base(pathConfig.Paths[0]) + "] \""
					} else {
						prefix = "\"[" + prefix + "] \""
					}
				} else {
					//pathConfig.Codec != "wzFormat"时, 为filebeat原格式, 不需要加前缀
					prefix = ""
				}

				cur := common.FilebeatInputsData{
					Hosts:           pathConfig.Hosts,
					Paths:           paths,
					Topic:           pathConfig.Topic,
					MultilineEnable: pathConfig.MultilineEnable,
					CustomField:     input.CustomField + ",container_name=" + cName,
					Codec:           pathConfig.Codec,
					Prefix:          prefix,
				}
				if pathConfig.MultilineEnable {
					cur.MultilinePattern = common.MultilineConfig{
						MulPattern: pathConfig.MultilinePattern.MulPattern,
						MulNegate:  pathConfig.MultilinePattern.MulNegate,
						MulMatch:   pathConfig.MultilinePattern.MulMatch,
					}
				}
				srcInputs = append(srcInputs, cur)
			}
		}
	}

	return &common.FilebeatInputConfigs{FBInputs: srcInputs}
}

func Parse(inputs []common.InputsData) (string, error) {
	inputsData := getInputsData(inputs)

	fbInputsYamlTpl, err := template.New("inputs.yml.template").ParseFiles(common.FilebeatInputsConfigTplPath)
	if err != nil {
		return "", err
	}
	buffer := bytes.NewBufferString("")
	if err := fbInputsYamlTpl.Execute(buffer, inputsData); err != nil {
		return "", fmt.Errorf("failed to excute fbInputsYamlTpl.Execute error:%w", err)
	}

	return buffer.String(), nil
}

func DecodeLogConfig(confStr string) (*common.LSCConfig, error) {
	if confStr != "" {
		conf := &common.LSCConfig{}
		err := json.Unmarshal([]byte(confStr), conf)
		if err != nil {
			conf = nil
		}
		return conf, err
	}
	return nil, nil
}

func Skip(anno map[string]string) bool {
	if lsc, exists := anno[common.LscAnnotationNeeded]; !exists || lsc != common.LogAnnotationNeededEnable {
		return true
	}
	if lscStr, exists := anno[common.LscAnnotationName]; !exists || strings.TrimSpace(lscStr) == "" {
		return true
	}

	return false
}

func Base(path string) string {
	if strings.HasSuffix(path, "/") {
		return "*"
	}

	return filepath.Base(path)
}

func LogConfigVaild(logconfig *common.LSCConfig) bool {
	vaild := true

	for _, containerConf := range logconfig.ContainerLogConfigs {
		for _, volumeConf := range containerConf {
			if volumeConf.Hosts == "" ||
				volumeConf.Topic == "" ||
				(volumeConf.LogType == common.FileMode && len(volumeConf.Paths) == 0) {
				vaild = false
				klog.Errorf("hosts:%s,topic:%s.\n", volumeConf.Hosts, volumeConf.Topic)
			}
		}
	}

	return vaild
}

type mountSpec struct {
	paths              []string
	firstVolumeMounted string
}

type QueryOrderSpec struct {
	mountSpec map[string]mountSpec
	order     []string
}

func CalculateHowToMount(logConfig common.VolumeLogConfig) QueryOrderSpec {
	result := map[string]mountSpec{}
	queryOrder := make([]string, 0)

	for vName, pathConfig := range logConfig {
		for _, logRelPath := range pathConfig.Paths {
			dir := filepath.Dir(logRelPath)
			if _, ok := result[dir]; !ok {
				result[dir] = mountSpec{
					paths:              make([]string, 0),
					firstVolumeMounted: vName,
				}
				queryOrder = append(queryOrder, dir)
			}
			newMountSpec := result[dir]
			if newMountSpec.firstVolumeMounted > vName {
				newMountSpec.firstVolumeMounted = vName
			}
			newMountSpec.paths = append(newMountSpec.paths, logRelPath)
			result[dir] = newMountSpec
		}
	}

	sort.Strings(queryOrder)

	return QueryOrderSpec{
		mountSpec: result,
		order:     queryOrder,
	}
}

func CalSubPath(queryOrderSpecInfo QueryOrderSpec, logPath, containerName string) string {
	basePath := ""
	volumeNameFirstMounted := ""
	for parent, child := range queryOrderSpecInfo.mountSpec {
		for _, path := range child.paths {
			if path == logPath {
				basePath = parent
				volumeNameFirstMounted = child.firstVolumeMounted
				break
			}
		}
	}

	idx := 0
	for i, parentPath := range queryOrderSpecInfo.order {
		if basePath == parentPath {
			idx = i
			break
		}
	}

	subPath := fmt.Sprintf("%s-%s-%d", volumeNameFirstMounted, containerName, idx)
	return subPath
}
