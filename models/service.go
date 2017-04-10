// mystack-controller api
// https://github.com/topfreegames/mystack-controller
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"bytes"
	"text/template"

	"github.com/topfreegames/mystack-controller/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

const serviceYaml = `
apiVersion: v1
kind: Service
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
  labels:
    mystack/routable: "true"
spec:
  selector:
    app: {{.Name}}
  ports:
    {{range .Ports}}
    - protocol: TCP
      port: {{.Port}}
      targetPort: {{.TargetPort}}
    {{end}}
  type: ClusterIP
`

//PortMap maps a port to a target por on service
type PortMap struct {
	Port       int
	TargetPort int
}

//Service represents a service
type Service struct {
	Name      string
	Namespace string
	Ports     []*PortMap
}

//NewService is the service ctor
func NewService(name, username string, ports []*PortMap) *Service {
	namespace := usernameToNamespace(username)
	return &Service{
		Name:      name,
		Namespace: namespace,
		Ports:     ports,
	}
}

//Expose exposes a deployment
func (s *Service) Expose(clientset kubernetes.Interface) (*v1.Service, error) {
	tmpl, err := template.New("expose").Parse(serviceYaml)
	if err != nil {
		return nil, errors.NewYamlError("parse yaml error", err)
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, s)
	if err != nil {
		return nil, errors.NewYamlError("parse yaml error", err)
	}

	decoder := api.Codecs.UniversalDecoder()
	obj, _, err := decoder.Decode(buf.Bytes(), nil, nil)
	if err != nil {
		return nil, errors.NewYamlError("parse yaml error", err)
	}

	src := obj.(*api.Service)
	dst := &v1.Service{}

	err = api.Scheme.Convert(src, dst, 0)
	if err != nil {
		return nil, errors.NewYamlError("parse yaml error", err)
	}

	service, err := clientset.CoreV1().Services(s.Namespace).Create(dst)

	if err != nil {
		return nil, errors.NewKubernetesError("create service error", err)
	}

	return service, nil
}

//Delete deletes service
func (s *Service) Delete(clientset kubernetes.Interface) error {
	deleteOptions := &v1.DeleteOptions{}

	err := clientset.CoreV1().Services(s.Namespace).Delete(s.Name, deleteOptions)
	if err != nil {
		return errors.NewKubernetesError("create service error", err)
	}

	return nil
}
