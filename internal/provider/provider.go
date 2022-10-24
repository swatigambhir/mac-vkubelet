package provider

import (
	"context"
	"path"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log/klogv2"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/scheme"
)

var log = klogv2.New(nil)

// TODO: Make these configurable
const CriSocketPath = "/run/containerd/containerd.sock"
const PodLogRoot = "/var/log/vk-cri/"
const PodVolRoot = "/run/vk-cri/volumes/"
const PodLogRootPerms = 0755
const PodVolRootPerms = 0755
const PodVolPerms = 0755
const PodSecretVolPerms = 0755
const PodSecretVolDir = "/secrets"
const PodSecretFilePerms = 0644
const PodConfigMapVolPerms = 0755
const PodConfigMapVolDir = "/configmaps"
const PodConfigMapFilePerms = 0644

type macPod struct {
	id string // This is the CRI Pod ID, not the UID from the Pod Spec
}

// mac provider - Intended for POC - implements the virtual-kubelet provider interface and manages pods in a mac runtime
type macprovider struct {
	config        *Opts
	nodeName      string
	startTime     time.Time
	kubernetesURL string
	podStatus     map[types.UID]macPod
}

// Create a new Provider
func NewProvider(ctx context.Context, opts *Opts) (*macprovider, error) {
	macp := macprovider{
		config:        opts,
		nodeName:      opts.NodeName,
		startTime:     time.Now(),
		kubernetesURL: opts.KubernetesURL,
		podStatus:     make(map[types.UID]macPod),
	}
	return &macp, nil
}

// Provider function to create a Pod,
func (p *macprovider) CreatePod(ctx context.Context, pod *corev1.Pod) error {

	podlog := log.
		WithField("podNamespace", pod.Namespace).
		WithField("podName", pod.Name)

	podlog.Info("Dummy CreatePod called")
	// TODO: Add Pod specific logging

	now := metav1.NewTime(time.Now())
	pod.Status = corev1.PodStatus{
		Phase:     corev1.PodRunning,
		HostIP:    "1.2.3.4",
		PodIP:     "2.3.4.5",
		StartTime: &now,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodInitialized,
				Status: corev1.ConditionTrue,
			},
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
			{
				Type:   corev1.PodScheduled,
				Status: corev1.ConditionTrue,
			},
		},
	}

	for _, container := range pod.Spec.Containers {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
			Name:         container.Name,
			Image:        container.Image,
			Ready:        true,
			RestartCount: 0,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: now,
				},
			},
			ContainerID: "12345",
		})

	}
	return nil
}

func (p *macprovider) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	// Provider function to delete a pod and its containers
	return errdefs.InvalidInput("delpod")
}

func (p *macprovider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	// Not required by VK
	return errdefs.InvalidInput("delpod")
}

func (p *macprovider) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	// Provider function to rreturn Pod Status
	return nil, errdefs.InvalidInput("getpod")
}

func (p *macprovider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	// Return the status of a Pod
	return nil, errdefs.NotFoundf("Pod %s in namespace %s could not be found on the node", name, namespace)
}

func (p *macprovider) GetPods(ctx context.Context) ([]*corev1.Pod, error) {
	// Implement GetPods to return all running pods
	var pods []*corev1.Pod
	return pods, nil
}

// Top-level command to start virtual-kubelet daemon.
func NewCommand(ctx context.Context, name string, opts *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: name + "provides a virtual kubelet interface for mac runners.",
		Long:  name + ` Runs a virtual kubelet daemon that makes it easier to manage mac machines with kubbectl like commands.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(ctx, opts)
		},
	}

	// Setup default opts.
	installFlags(cmd.Flags(), opts)

	return cmd
}

// Initialize Node
func runCommand(ctx context.Context, opts *Opts) error {
	log.Info("Initializing Nide....")
	ctx, cancelfunc := context.WithCancel(ctx)
	defer cancelfunc()

	// Setup a clientset for each of the API groups and versions
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: opts.KubeConfigPath},
		&clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return err
	}
	client, err := kubeclient.NewForConfig(cfg)

	if err != nil {
		return err
	}
	opts.KubernetesURL = cfg.Host

	// Create a shared informer factory for Kubernetes Pods assigned to this Node.
	podInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		client,
		1*time.Minute,
		nodeutil.PodInformerFilter(opts.NodeName),
	)
	podInformer := podInformerFactory.Core().V1().Pods()

	// Create informer factory for Kubernetes secrets and configmaps.
	informerFactory := informers.NewSharedInformerFactoryWithOptions(client, 1*time.Minute)
	secretInformerFactory := informerFactory.Core().V1().Secrets()
	configMapInformerFactory := informerFactory.Core().V1().ConfigMaps()
	serviceInformerFactory := informerFactory.Core().V1().Services()

	// Setup the macprovider.
	p, err := NewProvider(ctx, opts)
	if err != nil {
		return err
	}

	pNode := CreateNode(ctx, opts)

	np := node.NewNaiveNodeProvider()
	nodeLog := log.WithField("node", opts.NodeName)

	additionalOptions := []node.NodeControllerOpt{
		node.WithNodeStatusUpdateErrorHandler(func(ctx context.Context, err error) error {
			if !k8serrors.IsNotFound(err) {
				return err
			}
			nodeLog.Debug("node not found")
			newNode := pNode.DeepCopy()
			newNode.ResourceVersion = ""
			_, err = client.CoreV1().Nodes().Create(ctx, newNode, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			nodeLog.Debug("registered node")
			return nil
		}),
		node.WithNodeEnableLeaseV1(client.CoordinationV1().Leases("kube-node-lease"), 0),
	}
	// Set up the Node controller.
	nodeRunner, err := node.NewNodeController(
		np,
		pNode,
		client.CoreV1().Nodes(),
		additionalOptions...,
	)
	if err != nil {
		nodeLog.Fatal(errors.Wrap(err, "failed to set up node controller"))
	}

	// An event recorder is needed for the Pod controller.
	eb := record.NewBroadcaster()
	// Event recorder logging happens at debug level.
	eb.StartLogging(log.Debugf)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: client.CoreV1().Events(metav1.NamespaceAll)})
	// Set up the Pod controller.
	pc, err := node.NewPodController(node.PodControllerConfig{
		PodClient:         client.CoreV1(),
		PodInformer:       podInformer,
		EventRecorder:     eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: path.Join(pNode.Name, "pod-controller")}),
		Provider:          p,
		SecretInformer:    secretInformerFactory,
		ConfigMapInformer: configMapInformerFactory,
		ServiceInformer:   serviceInformerFactory,
	})
	if err != nil {
		return errors.Wrap(err, "error setting up pod controller")
	}

	// Start the informers.
	podInformerFactory.Start(ctx.Done())
	podInformerFactory.WaitForCacheSync(ctx.Done())
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	// Serve the kubelet API.
	cancelHTTP, err := setupKubeletServer(ctx, opts, func(context.Context) ([]*corev1.Pod, error) {
		return podInformer.Lister().List(labels.Everything())
	})
	if err != nil {
		return err
	}
	defer cancelHTTP()

	//Start the Pod controller.
	go func() {
		if err := pc.Run(ctx, opts.PodSyncWorkers); err != nil && errors.Cause(err) != context.Canceled {
			nodeLog.Fatal(errors.Wrap(err, "failed to start pod controller"))
		}
	}()

	// Start the Node controller.
	go func() {
		if err := nodeRunner.Run(ctx); err != nil {
			nodeLog.Fatal(errors.Wrap(err, "failed to start node controller"))
		}
	}()

	// If we got here, set Node condition Ready.
	setNodeReady(pNode)
	if err := np.UpdateStatus(ctx, pNode); err != nil {
		return errors.Wrap(err, "error marking the node as ready")
	}
	nodeLog.Info("macnode initialized")

	<-ctx.Done()
	return nil
}

func setNodeReady(n *corev1.Node) {
	for i, c := range n.Status.Conditions {
		if c.Type != "Ready" {
			continue
		}
		c.Message = "provider ready"
		c.Reason = "Virtual Kubelet Ready"
		c.Status = corev1.ConditionTrue
		c.LastHeartbeatTime = metav1.Now()
		c.LastTransitionTime = metav1.Now()
		n.Status.Conditions[i] = c
		return
	}
}
func installFlags(flags *pflag.FlagSet, c *Opts) {
	flags.StringVar(&c.KubeConfigPath, "kubeconfig", "", "cluster client configuration")
	flags.StringVar(&c.NodeName, "nodename", DefaultNodeName, "The value to be set as the Node name")
	flags.IntVar(&c.PodSyncWorkers, "pod-sync-workers", DefaultPodSyncWorkers, "The number of Pod synchronization workers")
	flags.StringVar(&c.KubeNamespace, "namespace", DefaultKubeNamespace, "The Kubernetes namespace")

}
