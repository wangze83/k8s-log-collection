package app

import (
	"corp.wz.net/opsdev/log-collection/pkg/admission/sidecar"
	kube "corp.wz.net/opsdev/log-collection/pkg/common/kubernetes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"net/http"
)

func NewCommand() *cobra.Command {
	opts := Options{}
	cmd := &cobra.Command{
		Use:  "log-logsidecar-injector",
		Long: "create cm for pod that collect logs by logsidecar",
		RunE: func(c *cobra.Command, args []string) error {
			if err := start(opts); err != nil {
				klog.Error(fmt.Sprintf("run command err: %w", err))
				return err
			}
			return nil
		},
	}

	fs := cmd.Flags()
	opts.AddFlags(fs)

	return cmd
}

func start(opts Options) error {
	restConfig, err := kube.NewRestConfig(opts.KubeConfig)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	kubeDynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	groupResources, err := restmapper.GetAPIGroupResources(kubeClient.Discovery())
	if err != nil {
		klog.Errorf("failed to get gvr,error=%v\n", err)
		return err
	}
	rm := restmapper.NewDiscoveryRESTMapper(groupResources)

	mux := http.NewServeMux()
	h := handler{
		sidecar.NewInjector(kubeClient, kubeDynamicClient, rm),
	}
	mux.HandleFunc("/mutate", h.mutate)

	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", opts.Port),
		Handler:        mux,
		ReadTimeout:    opts.ReadTimeout,
		WriteTimeout:   opts.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	return s.ListenAndServeTLS(opts.CertFile, opts.KeyFile)
}

type injectorInterface interface {
	Mutate(pod *corev1.Pod, namespace string)
}

type handler struct {
	injector injectorInterface
}

func (h *handler) mutate(w http.ResponseWriter, r *http.Request) {
	resp, err := h.getResponse(r.Body)
	if err != nil {
		klog.Error(err)
	}

	if _, err := w.Write(resp); err != nil {
		failIfError(w, err)
	}
}

func (h *handler) getResponse(rawAdmissionReview io.Reader) ([]byte, error) {
	admissionReview := &v1beta1.AdmissionReview{}
	err := json.NewDecoder(rawAdmissionReview).Decode(admissionReview)
	if err != nil {
		return nil, err
	}
	if admissionReview.Request == nil {
		return nil, errors.New("request is null")
	}

	h.performAdmissionReview(admissionReview)
	resp, err := json.Marshal(admissionReview)

	return resp, err
}

func (h *handler) performAdmissionReview(admissionReview *v1beta1.AdmissionReview) {
	pod, err := getPod(admissionReview)
	if err != nil {
		admissionReview.Response = toAdmissionResponse(err)
		return
	}

	h.injector.Mutate(pod, admissionReview.Request.Namespace)
	patch, err := createPatch(admissionReview.Request.Object.Raw, pod)
	if err != nil {
		admissionReview.Response = toAdmissionResponse(err)
		return
	}

	jsonPatch := v1beta1.PatchTypeJSONPatch
	admissionReview.Response = &v1beta1.AdmissionResponse{
		Allowed:   true,
		PatchType: &jsonPatch,
		Patch:     patch,
		Result: &metav1.Status{
			Status: "Success",
		},
	}
}

func getPod(admissionReview *v1beta1.AdmissionReview) (*corev1.Pod, error) {
	var pod corev1.Pod
	if admissionReview.Request == nil {
		return nil, errors.New("request is nil")
	}
	if admissionReview.Request.Object.Raw == nil {
		return nil, errors.New("request object raw is nil")
	}
	err := json.Unmarshal(admissionReview.Request.Object.Raw, &pod)
	if err != nil {
		return nil, err
	}
	return &pod, nil
}

func createPatch(raw []byte, mutated runtime.Object) ([]byte, error) {
	mu, err := json.Marshal(mutated)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.CreatePatch(raw, mu)
	if err != nil {
		return nil, err
	}
	if len(patch) > 0 {
		return json.Marshal(patch)
	}
	return nil, nil
}

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
			Status:  "Fail",
		},
	}
}

func failIfError(w http.ResponseWriter, err error) {
	if err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
