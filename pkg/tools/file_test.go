package tools

import "testing"

func TestGetLastestUpdateFile(t *testing.T) {
	type args struct {
		dir string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "test1",
			args:    args{"/log-collection1/charts/daemonset-controller/templates"},
			want:    "daemonset.yaml",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetLastestUpdateFile(tt.args.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLastestUpdateFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetLastestUpdateFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}
