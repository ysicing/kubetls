package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/ergoapi/util/exhash"
	"github.com/ergoapi/util/exmap"
	"github.com/ergoapi/util/exstr"
	"github.com/ergoapi/util/version"
	"github.com/ergoapi/util/zos"
	"github.com/sirupsen/logrus"
	"github.com/ysicing/kubetls/constants"
	"github.com/ysicing/kubetls/internal/kube"
	"github.com/ysicing/kubetls/pkg/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var K8SClient kubernetes.Interface

func init() {
	var err error
	kubecfg := &kube.ClientConfig{}
	K8SClient, err = kube.New(kubecfg)
	if err != nil {
		panic(err)
	}
}

func Pods() {
	logrus.Debug("pods")
}

func KV() string {
	kv, err := K8SClient.Discovery().ServerVersion()
	if err != nil {
		return "unknow"
	}
	return kv.String()
}

func Check(ns string) error {
	nsresp, err := K8SClient.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	kubetlsfile := "/conf/kubetls.yaml"

	if zos.IsMacOS() {
		kubetlsfile = "./kubetls.yaml"
	}
	cfg, err := config.LoadFile(kubetlsfile)
	if err != nil {
		return err
	}
	// 不存在 或者 存在版本低
	if !exmap.CheckLabel(nsresp.Annotations, constants.KubeCR) || version.LT(cfg.CR.Version, exmap.GetLabelValue(nsresp.Annotations, constants.KubeCR)) {
		logrus.Infof("namespace: %s, kubetls init", ns)
		// 创建 imagePullSecrets
		var imagePullSecrets []v1.LocalObjectReference
		for _, i := range cfg.CR.Secrets {
			if err := secret(ns, i); err != nil {
				logrus.Errorf("[%s] sync secret %v, type: %v, err: %v", ns, i.Name, i.Type, err)
				continue
			}
			imagePullSecrets = append(imagePullSecrets, v1.LocalObjectReference{
				Name: i.Name,
			})
		}
		// 创建sa绑定
		if len(imagePullSecrets) > 0 {
			if err := patch(ns, imagePullSecrets); err == nil {
				nsstatus(ns, constants.KubeCR, cfg.CR.Version)
			}
		}
	}
	if !exmap.CheckLabel(nsresp.Annotations, constants.KubeTLS) || version.LT(cfg.TLS.Version, exmap.GetLabelValue(nsresp.Annotations, constants.KubeTLS)) {
		for _, i := range cfg.TLS.Secrets {
			if err := secret(ns, i); err != nil {
				logrus.Errorf("[%s] sync secret %v, type: %v, err: %v", ns, i.Name, i.Type, err)
			}
		}
		nsstatus(ns, constants.KubeTLS, cfg.TLS.Version)
	}
	return nil
}

type NamespaceControlller struct {
	informerFactory informers.SharedInformerFactory
	nsInformer      coreinformers.NamespaceInformer
	nodeInformer    coreinformers.NodeInformer
}

func (c *NamespaceControlller) nscreate(obj interface{}) {
	ns := obj.(*v1.Namespace)
	if exstr.KubeBlacklist(ns.Name) {
		return
	}

	kubetlsfile := "/conf/kubetls.yaml"

	if zos.IsMacOS() {
		kubetlsfile = "./kubetls.yaml"
	}
	cfg, err := config.LoadFile(kubetlsfile)
	if err != nil {
		return
	}

	// 不存在 或者 存在版本低
	if !exmap.CheckLabel(ns.Annotations, constants.KubeCR) || version.LT(cfg.CR.Version, exmap.GetLabelValue(ns.Annotations, constants.KubeCR)) {
		logrus.Infof("namespace: %s, kubetls init", ns.Name)
		// 创建 imagePullSecrets
		var imagePullSecrets []v1.LocalObjectReference
		for _, i := range cfg.CR.Secrets {
			if err := secret(ns.Name, i); err != nil {
				logrus.Errorf("[%s] sync secret %v, type: %v, err: %v", ns.Name, i.Name, i.Type, err)
				continue
			}
			imagePullSecrets = append(imagePullSecrets, v1.LocalObjectReference{
				Name: i.Name,
			})
		}
		// 创建sa绑定
		if err := patch(ns.Name, imagePullSecrets); err == nil {
			nsstatus(ns.Name, constants.KubeCR, cfg.CR.Version)
		}
	}
	if !exmap.CheckLabel(ns.Annotations, constants.KubeTLS) || version.LT(cfg.TLS.Version, exmap.GetLabelValue(ns.Annotations, constants.KubeTLS)) {
		for _, i := range cfg.TLS.Secrets {
			if err := secret(ns.Name, i); err != nil {
				logrus.Errorf("[%s] sync secret %v, type: %v, err: %v", ns.Name, i.Name, i.Type, err)
			}
		}
		nsstatus(ns.Name, constants.KubeTLS, cfg.TLS.Version)
	}
}

func (c *NamespaceControlller) nsdelete(obj interface{}) {
	ns := obj.(*v1.Namespace)
	logrus.Infof("namespace delete: %s", ns.Name)
}

func (c *NamespaceControlller) nsupdate(oldobj interface{}, newobj interface{}) {
	oldns := oldobj.(*v1.Namespace)
	newns := newobj.(*v1.Namespace)
	logrus.Infof("namespace %v annotations update: %s --> %s", newns.GetName(), oldns.GetAnnotations(), newns.GetAnnotations())
}

func (c *NamespaceControlller) nodeadd(obj interface{}) {
	node := obj.(*v1.Node)
	msg := fmt.Sprintf("新增节点: %s, ip: %s", node.Name, node.Status.Addresses[0].Address)
	logrus.Info(msg)
	// feishu.Send(fmt.Sprintf("%s %s", ztime.NowFormat(), msg))
}

func (c *NamespaceControlller) nodedel(obj interface{}) {
	node := obj.(*v1.Node)
	msg := fmt.Sprintf("下线节点: %s, ip: %s", node.Name, node.Status.Addresses[0].Address)
	logrus.Info(msg)
	// feishu.Send(fmt.Sprintf("%s %s", ztime.NowFormat(), msg))
}

func (c *NamespaceControlller) Run(stopCh chan struct{}) error {
	// Starts all the shared informers that have been created by the factory so
	// far.
	c.informerFactory.Start(stopCh)
	// wait for the initial synchronization of the local cache.
	if !cache.WaitForCacheSync(stopCh, c.nsInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to sync")
	}
	return nil
}

func NewNamespaceControlller(i informers.SharedInformerFactory) *NamespaceControlller {
	nsInformer := i.Core().V1().Namespaces()
	nodeInformer := i.Core().V1().Nodes()
	c := &NamespaceControlller{
		informerFactory: i,
		nsInformer:      nsInformer,
		nodeInformer:    nodeInformer,
	}
	nsInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.nscreate,
			DeleteFunc: c.nsdelete,
			UpdateFunc: c.nsupdate,
		},
	)
	nodeInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.nodeadd,
			DeleteFunc: c.nodedel,
		},
	)
	return c
}

func secret(ns string, cfg *config.Secret) error {
	kubeclient := K8SClient.CoreV1().Secrets(ns)
	s, err := kubeclient.Get(context.TODO(), cfg.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("err: %v", err)
		return err
	}
	logrus.Debugf("ns: %v, name: %v, type: %v", ns, cfg.Name, cfg.Type)
	if errors.IsNotFound(err) {
		object := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: cfg.Name,
				Labels: map[string]string{
					constants.KubeCM: "kubetls",
				},
			},
		}
		if cfg.Type == "dockercfg" {
			object.Type = v1.SecretTypeDockercfg
			data, _ := exhash.B64Decode(cfg.Data)
			object.Data = map[string][]byte{
				".dockercfg": []byte(data),
			}
		}

		if cfg.Type == "dockerconfigjson" {
			object.Type = v1.SecretTypeDockerConfigJson
			data, _ := exhash.B64Decode(cfg.Data)
			// object.Data = map[string][]byte{
			// 	".dockerconfigjson": []byte(data),
			// }
			object.StringData = map[string]string{
				".dockerconfigjson": string(data),
			}
		}

		if cfg.Type == "tls" {
			object.Type = v1.SecretType("kubernetes.io/tls")
			cert, _ := exhash.B64Decode(cfg.Crt)
			key, _ := exhash.B64Decode(cfg.Key)
			object.Data = map[string][]byte{
				"tls.crt": []byte(cert),
				"tls.key": []byte(key),
			}
		}

		_, err := kubeclient.Create(context.TODO(), object, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				return nil
			}
			logrus.Errorf("create err: %v", err)
			return err
		}
		return nil
	}

	if cfg.Type == "dockercfg" {
		s.Type = v1.SecretTypeDockercfg
		data, _ := exhash.B64Decode(cfg.Data)
		s.Data = map[string][]byte{
			".dockercfg": []byte(data),
		}
	}

	if cfg.Type == "dockerconfigjson" {
		s.Type = v1.SecretTypeDockercfg
		s.Data = map[string][]byte{
			".dockerconfigjson": []byte(cfg.Data),
		}
	}

	if cfg.Type == "tls" {
		s.Type = v1.SecretType("kubernetes.io/tls")
		cert, _ := exhash.B64Decode(cfg.Crt)
		key, _ := exhash.B64Decode(cfg.Key)
		s.StringData = map[string]string{
			"tls.crt": cert,
			"tls.key": key,
		}
	}

	if _, err := kubeclient.Update(context.TODO(), s, metav1.UpdateOptions{}); err != nil {
		logrus.Errorf("update err: %v", err)
		if err := kubeclient.Delete(context.TODO(), s.Name, metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("delete err: %v", err)
		}
		return secret(ns, cfg)
	}
	return nil
}

func patch(ns string, imagePullSecrets []v1.LocalObjectReference) error {
	kubeclient := K8SClient.CoreV1().ServiceAccounts(ns)
	s, err := kubeclient.Get(context.TODO(), "default", metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if errors.IsNotFound(err) {
		time.Sleep(time.Second * 5)
		patch(ns, imagePullSecrets)
		return nil
	}
	s.ImagePullSecrets = imagePullSecrets
	_, err = kubeclient.Update(context.TODO(), s, metav1.UpdateOptions{})
	return err
}

func nsstatus(ns, key, value string) {
	kubeclient := K8SClient.CoreV1().Namespaces()
	nsresp, err := kubeclient.Get(context.TODO(), ns, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("get ns %s, key: %v [%v]  err: %v", ns, key, value, err)
		return
	}

	nsresp.Annotations = exmap.MergeLabels(nsresp.Annotations, map[string]string{
		key: value,
	})

	_, err = kubeclient.Update(context.TODO(), nsresp, metav1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update ns %v, an: %v [%v]  err: %v", ns, key, value, err)
	}
}
