package domain

type HealthStatus struct {
	Name   string
	Status string
}

func NewHealthStatus(name string, status string) HealthStatus {
	return HealthStatus{
		Name:   name,
		Status: status,
	}
}
