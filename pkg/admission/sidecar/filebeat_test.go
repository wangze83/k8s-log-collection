package sidecar

import (
	"corp.wz.net/opsdev/log-collection/pkg/common"
	"fmt"
	"testing"
)

func Test_getLogAndVolPath(t *testing.T) {
	type args struct {
		srcPath       string
		collectorType common.LogCollectorType
		subPath       string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 string
	}{
		// TODO: Add test cases.
		{
			name: "controller not contain *",
			args: args{
				srcPath:       "/data/zw-test/zw.log",
				collectorType: common.SidecarMode,
				subPath:       "container-1-1",
			},
			want:  "/data/sidecar-log/container-1-1/zw.log",
			want1: "/data/zw-test",
		},
		{
			name: "controller not contain *",
			args: args{
				srcPath:       "/data/zw-test/zw.log",
				collectorType: common.DaemonsetMode,
				subPath:       "container-1-1",
			},
			want:  "/data/controller-log/container-1-1/zw.log",
			want1: "/data/zw-test",
		},
		{
			name: "controller contain *",
			args: args{
				srcPath:       "/data/zw-test/*.log",
				collectorType: common.SidecarMode,
				subPath:       "container-1-1",
			},
			want:  "/data/sidecar-log/container-1-1/*.log",
			want1: "/data/zw-test",
		},
		{
			name: "controller contain *",
			args: args{
				srcPath:       "/data/zw-test/*.log",
				collectorType: common.DaemonsetMode,
				subPath:       "container-1-1",
			},
			want:  "/data/controller-log/container-1-1/*.log",
			want1: "/data/zw-test",
		},
		{
			name: "dir not contain *",
			args: args{
				srcPath:       "/data/zw-test/",
				collectorType: common.DaemonsetMode,
				subPath:       "container-1-1",
			},
			want:  "/data/controller-log/container-1-1/",
			want1: "/data/zw-test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetLogAndVolPath(tt.args.srcPath, tt.args.collectorType, tt.args.subPath)
			fmt.Println(got, got1)
			if got != tt.want {
				t.Errorf("getLogAndVolPath() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getLogAndVolPath() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
