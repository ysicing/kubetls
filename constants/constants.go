package constants

import "fmt"

var (
	Commit  = "unknown"
	Date    = "unknown"
	Release = "unknown"
	Version = fmt.Sprintf("%s-%s-%s", Release, Date, Commit)
)

var (
	KubeCR  = "k8s.ysicing.work/kubecr"
	KubeTLS = "k8s.ysicing.work/kubetls"
	KubeCM  = "k8s.ysicing.work/component"
)
