package models

// DaskContext is the set of parameters to configures this instance
type DaskContext struct {
	Daemon       bool
	Namespace    string
	Name         string
	ServiceType  string
	Port         int
	BokehPort    int
	Replicas     int
	Image        string
	Repository   string
	Tag          string
	PullSecret   string
	PullPolicy   string
	NodeSelector string
	Affinity     string
	Tolerations  string
}
