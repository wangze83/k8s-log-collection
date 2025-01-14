package filebeat

import (
	"reflect"
	"testing"

	"corp.wz.net/opsdev/log-collection/pkg/common"
)

func TestSkip(t *testing.T) {
	type args struct {
		anno map[string]string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{map[string]string{"logging.io/logsidecar-config": "{\"containerLogConfigs\":{\"wz\":{\"datavolume-0\":{\"log_collector_type\":1,\"log_type\":0,\"multiline_enable\":false,\"topic\":\"wz_log\",\"hosts\":\"10.1.1.1:39092\"},\"datavolume-1\":{\"log_collector_type\":1,\"log_type\":1,\"paths\":[\"/var/log/wz/wz.log\",\"/var/log/wz/wz_orm.log\"],\"multiline_enable\":false,\"topic\":\"wz_log\",\"hosts\":\"10.1.1.1:39092\"}}}}", "logsidecar-inject.logging-enable": "enable"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Skip(tt.args.anno); got != tt.want {
				t.Errorf("Skip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCollectLog(t *testing.T) {
	type args struct {
		logconfig common.LSCConfig
		mode      common.LogCollectorType
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "log collection by daemonset container",
			args: args{
				logconfig: common.LSCConfig{
					ContainerLogConfigs: map[string]common.VolumeLogConfig{"app-container": {
						"datavolume1": common.VolumePathConfig{
							LogCollectorType: 1,
							Paths:            []string{"/data/log/*.log"},
							Topic:            "filebeat-test",
							Hosts:            "10.1.1.1:39092",
							MultilineEnable:  false,
							LogType:          1,
						}}},
					SidecarResources: common.ResourceRequirements{},
				},
				mode: 1,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCollectLog(tt.args.logconfig, tt.args.mode); got != tt.want {
				t.Errorf("IsCollectLog() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateHowToMount(t *testing.T) {
	type args struct {
		logConfig common.VolumeLogConfig
	}
	tests := []struct {
		name string
		args args
		want QueryOrderSpec
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{common.VolumeLogConfig{
				"datavolume-0": common.VolumePathConfig{
					LogCollectorType: 1,
					Paths: []string{
						"/data/log/*.log",
						"/data1/log/*.log",
					},
					Topic:            "filebeat-test",
					Hosts:            "10.1.1.1:39092",
					MultilineEnable:  false,
					MultilinePattern: common.MultilineConfig{},
					LogType:          1,
				},
			}},
			want: QueryOrderSpec{
				mountSpec: map[string]mountSpec{
					"/data/log": {
						paths:              []string{"/data/log/*.log"},
						firstVolumeMounted: "datavolume-0",
					},
					"/data1/log": {
						paths:              []string{"/data1/log/*.log"},
						firstVolumeMounted: "datavolume-0",
					},
				},
				order: []string{
					"/data/log",
					"/data1/log",
				},
			},
		},
		{
			name: "test2",
			args: args{common.VolumeLogConfig{
				"datavolume-0": common.VolumePathConfig{
					LogCollectorType: 0,
					Paths: []string{
						"/data/nginx/logs/score_shop/app/apiaccess.log",
					},
					Topic:            "qa_score_shop_access",
					Hosts:            "10.1.1.1:39092",
					MultilineEnable:  false,
					MultilinePattern: common.MultilineConfig{},
					LogType:          1,
					Codec:            "json",
					Prefix:           "",
					Cluster:          "",
				},
				"datavolume-1": common.VolumePathConfig{
					LogCollectorType: 0,
					Paths: []string{
						"/data/nginx/logs/score_shop/web/score_shop-access.log",
						"/data/nginx/logs/score_shop/web/score_shop-error.log",
					},
					Topic:            "qa_score_shop_nginx",
					Hosts:            "10.1.1.1:39092",
					MultilineEnable:  false,
					MultilinePattern: common.MultilineConfig{},
					LogType:          1,
					Codec:            "",
					Prefix:           "",
					Cluster:          "",
				},
				"datavolume-2": common.VolumePathConfig{
					LogCollectorType: 0,
					Paths: []string{
						"/data/sdk/score_utils/logs/*",
						"/data/nginx/logs/score_shop/app/utils_*",
					},
					Topic:            "qa_score_shop_general",
					Hosts:            "10.1.1.1:39092",
					MultilineEnable:  false,
					MultilinePattern: common.MultilineConfig{},
					LogType:          1,
					Codec:            "",
					Prefix:           "",
					Cluster:          "",
				},
				"datavolume-3": common.VolumePathConfig{
					LogCollectorType: 0,
					Paths: []string{
						"/data/nginx/logs/score_shop/app/apipoint.log",
					},
					Topic:            "qa_score_shop_point",
					Hosts:            "10.1.1.1:39092",
					MultilineEnable:  false,
					MultilinePattern: common.MultilineConfig{},
					LogType:          1,
					Codec:            "",
					Prefix:           "",
					Cluster:          "",
				},
			}},
			want: QueryOrderSpec{
				mountSpec: map[string]mountSpec{
					"/data/nginx/logs/score_shop/app": {
						paths: []string{
							"/data/nginx/logs/score_shop/app/apiaccess.log",
							"/data/nginx/logs/score_shop/app/utils_*",
							"/data/nginx/logs/score_shop/app/apipoint.log",
						},
						firstVolumeMounted: "datavolume-0",
					},
					"/data/nginx/logs/score_shop/web": {
						paths: []string{
							"/data/nginx/logs/score_shop/web/score_shop-access.log",
							"/data/nginx/logs/score_shop/web/score_shop-error.log",
						},
						firstVolumeMounted: "datavolume-1",
					},
					"/data/sdk/score_utils/logs": {
						paths: []string{
							"/data/sdk/score_utils/logs/*",
						},
						firstVolumeMounted: "datavolume-2",
					},
				},
				order: []string{
					"/data/nginx/logs/score_shop/app",
					"/data/nginx/logs/score_shop/web",
					"/data/sdk/score_utils/logs",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateHowToMount(tt.args.logConfig)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CalculateHowToMount() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalSubPath(t *testing.T) {
	type args struct {
		queryOrderSpecInfo QueryOrderSpec
		logPath            string
		containerName      string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{
				queryOrderSpecInfo: QueryOrderSpec{
					mountSpec: map[string]mountSpec{
						"/data/log": {
							paths:              []string{"/data/log/*.log"},
							firstVolumeMounted: "datavolume-0",
						},
						"/data1/log": {
							paths:              []string{"/data1/log/*.log"},
							firstVolumeMounted: "datavolume-1",
						},
					},
					order: []string{
						"/data/log",
						"/data1/log",
					},
				},
				logPath:       "/data/log/*.log",
				containerName: "app-container",
			},
			want: "datavolume-0-app-container-0",
		},
		{
			name: "test2",
			args: args{
				queryOrderSpecInfo: QueryOrderSpec{
					mountSpec: map[string]mountSpec{
						"/data/log": {
							paths:              []string{"/data/log/*.log"},
							firstVolumeMounted: "datavolume-0",
						},
						"/data1/log": {
							paths:              []string{"/data1/log/*.log"},
							firstVolumeMounted: "datavolume-1",
						},
					},
					order: []string{
						"/data/log",
						"/data1/log",
					},
				},
				logPath:       "/data1/log/*.log",
				containerName: "app-container",
			},
			want: "datavolume-1-app-container-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalSubPath(tt.args.queryOrderSpecInfo, tt.args.logPath, tt.args.containerName); got != tt.want {
				t.Errorf("CalSubPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	type args struct {
		inputs []common.InputsData
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{
				[]common.InputsData{
					{
						ContainerLogConfigs: common.ContainerLogConfigs{
							"container1": common.VolumeLogConfig{
								"volume1": common.VolumePathConfig{
									LogCollectorType: 0,
									Paths:            []string{"/log/*", "/data1/*.log"},
									Topic:            "test",
									Hosts:            "127.0.0.1",
									MultilineEnable:  false,
									MultilinePattern: common.MultilineConfig{
										MulPattern: "^[[:space:]]",
										MulNegate:  "false",
										MulMatch:   "after",
									},
									LogType: 1,
								},
							},
							"container2": common.VolumeLogConfig{
								"volume1": common.VolumePathConfig{
									LogCollectorType: 0,
									Paths:            []string{"/log/*", "/data1/*.log"},
									Topic:            "test",
									Hosts:            "127.0.0.1",
									MultilineEnable:  false,
									MultilinePattern: common.MultilineConfig{
										MulPattern: "^[[:space:]]",
										MulNegate:  "false",
										MulMatch:   "after",
									},
									LogType: 1,
								},
							},
						},
						CustomField: "test",
					},
				},
			},
			want: `
- type: log
  enabled: true
  symlinks: true
  paths:
  - /data/sidecar-log/volume1-container1-0/*
  tail_files: true
  scan_frequency: 10s
  backoff: 1s
  max_backoff: 10s
  fields:
	app_field: test
  output:
	hosts: 127.0.0.1
	topic: test
- type: log
  enabled: true
  symlinks: true
  paths:
  - /data/sidecar-log/volume1-container1-0/*
  tail_files: true
  scan_frequency: 10s
  backoff: 1s
  max_backoff: 10s
  fields:
	app_field: test
  output:
	hosts: 127.0.0.1
	topic: test`,
			wantErr: false,
		},
		{
			name: "test2",
			args: args{
				[]common.InputsData{
					{
						ContainerLogConfigs: common.ContainerLogConfigs{
							"container1": common.VolumeLogConfig{
								"volume1": common.VolumePathConfig{
									LogCollectorType: 0,
									Paths:            []string{"/log/*", "/data1/*.log"},
									Topic:            "test",
									Hosts:            "127.0.0.1",
									MultilineEnable:  false,
									MultilinePattern: common.MultilineConfig{
										MulPattern: "^[[:space:]]",
										MulNegate:  "false",
										MulMatch:   "after",
									},
									LogType: 1,
									Codec:   "format",
								},
							},
							"container2": common.VolumeLogConfig{
								"volume1": common.VolumePathConfig{
									LogCollectorType: 0,
									Paths:            []string{"/log/*", "/data1/*.log"},
									Topic:            "test",
									Hosts:            "127.0.0.1",
									MultilineEnable:  false,
									MultilinePattern: common.MultilineConfig{
										MulPattern: "^[[:space:]]",
										MulNegate:  "false",
										MulMatch:   "after",
									},
									LogType: 1,
									Codec:   "format",
								},
							},
						},
						CustomField: "test",
					},
				},
			},
			want: `
- type: log
  enabled: true
  symlinks: true
  paths:
  - /data/sidecar-log/volume1-container1-0/*
  tail_files: true
  scan_frequency: 10s
  backoff: 1s
  max_backoff: 10s
  fields:
	app_field: test
  output:
	hosts: 127.0.0.1
	topic: test
	codec: format
	prefix: ""
- type: log
  enabled: true
  symlinks: true
  paths:
  - /data/sidecar-log/volume1-container1-0/*
  tail_files: true
  scan_frequency: 10s
  backoff: 1s
  max_backoff: 10s
  fields:
	app_field: test
  output:
	hosts: 127.0.0.1
	topic: test
	codec: format
	prefix: ""`,
			wantErr: false,
		},
		{
			name: "test3",
			args: args{
				[]common.InputsData{
					{
						ContainerLogConfigs: common.ContainerLogConfigs{
							"container1": common.VolumeLogConfig{
								"volume1": common.VolumePathConfig{
									LogCollectorType: 0,
									Paths:            []string{"/log/*", "/data1/*.log"},
									Topic:            "test",
									Hosts:            "127.0.0.1",
									MultilineEnable:  false,
									MultilinePattern: common.MultilineConfig{
										MulPattern: "^[[:space:]]",
										MulNegate:  "false",
										MulMatch:   "after",
									},
									LogType: 1,
								},
							},
							"container2": common.VolumeLogConfig{
								"volume1": common.VolumePathConfig{
									LogCollectorType: 0,
									Paths:            []string{"/log/*", "/data1/*.log"},
									Topic:            "test",
									Hosts:            "127.0.0.1",
									MultilineEnable:  false,
									MultilinePattern: common.MultilineConfig{
										MulPattern: "^[[:space:]]",
										MulNegate:  "false",
										MulMatch:   "after",
									},
									LogType: 1,
									Codec:   "format",
								},
							},
						},
						CustomField: "test",
					},
				},
			},
			want: `
- type: log
  enabled: true
  symlinks: true
  paths:
  - /data/sidecar-log/volume1-container1-0/*
  tail_files: true
  scan_frequency: 10s
  backoff: 1s
  max_backoff: 10s
  fields:
	app_field: test
  output:
	hosts: 127.0.0.1
	topic: test
- type: log
  enabled: true
  symlinks: true
  paths:
  - /data/sidecar-log/volume1-container1-0/*
  tail_files: true
  scan_frequency: 10s
  backoff: 1s
  max_backoff: 10s
  fields:
	app_field: test
  output:
	hosts: 127.0.0.1
	topic: test
	codec: format
	prefix: ""`,
			wantErr: false,
		},
		{
			name: "test4",
			args: args{
				[]common.InputsData{
					{
						ContainerLogConfigs: common.ContainerLogConfigs{
							"container1": common.VolumeLogConfig{
								"volume1": common.VolumePathConfig{
									LogCollectorType: 0,
									Paths:            []string{"/log/*", "/data1/*.log"},
									Topic:            "test",
									Hosts:            "127.0.0.1",
									MultilineEnable:  false,
									MultilinePattern: common.MultilineConfig{
										MulPattern: "^[[:space:]]",
										MulNegate:  "false",
										MulMatch:   "after",
									},
									LogType: 1,
									Codec:   "format",
								},
							},
							"container2": common.VolumeLogConfig{
								"volume1": common.VolumePathConfig{
									LogCollectorType: 0,
									Paths:            []string{"/log/*", "/data1/*.log"},
									Topic:            "test",
									Hosts:            "127.0.0.1",
									MultilineEnable:  false,
									MultilinePattern: common.MultilineConfig{
										MulPattern: "^[[:space:]]",
										MulNegate:  "false",
										MulMatch:   "after",
									},
									LogType: 1,
								},
							},
						},
						CustomField: "test",
					},
				},
			},
			want: `
- type: log
  enabled: true
  symlinks: true
  paths:
  - /data/sidecar-log/volume1-container1-0/*
  tail_files: true
  scan_frequency: 10s
  backoff: 1s
  max_backoff: 10s
  fields:
	app_field: test
  output:
	hosts: 127.0.0.1
	topic: test
	codec: format
	prefix: ""
- type: log
  enabled: true
  symlinks: true
  paths:
  - /data/sidecar-log/volume1-container1-0/*
  tail_files: true
  scan_frequency: 10s
  backoff: 1s
  max_backoff: 10s
  fields:
	app_field: test
  output:
	hosts: 127.0.0.1
	topic: test`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Parse() got = %v, want %v", got, tt.want)
			}
		})
	}
}
