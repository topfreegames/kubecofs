// mystack-controller api
// +build unit
// https://github.com/topfreegames/mystack-controller
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/mystack-controller/models"

	"database/sql"
	"github.com/jmoiron/sqlx"
	mTest "github.com/topfreegames/mystack-controller/testing"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/labels"
)

var _ = Describe("Cluster", func() {
	const (
		yaml1 = `
setup:
  image: setup-img
services:
  test0:
    image: svc1
    ports: 
      - "5000"
      - "5001:5002"
    readiness-probe:
      command:
        - echo
        - ready
apps:
  test1:
    image: app1
    ports: 
      - "5000"
      - "5001:5002"
  test2:
    image: app2
    ports: 
      - "5000"
      - "5001:5002"
  test3:
    image: app3
    ports: 
      - "5000"
      - "5001:5002"
    env:
      - name: VARIABLE_1
        value: 100
`
		yaml2 = `
setup:
  image: setup-img
  timeout-seconds: 180
  period-seconds: 10
services:
  test0:
    image: svc1
    ports: 
      - "5000"
      - "5001:5002"
    readiness-probe:
      command:
        - echo
        - ready
      period-seconds: 10
      start-deployment-timeout-seconds: 180
apps:
  test1:
    image: app1
    ports: 
      - "5000"
      - "5001:5002"
  test2:
    image: app2
    ports: 
      - "5000"
      - "5001:5002"
  test3:
    image: app3
    ports: 
      - "5000"
      - "5001:5002"
    env:
      - name: VARIABLE_1
        value: 100
`
		invalidYaml1 = `
services:
  postgres:
    image: postgres:1.0
    ports:
      - 8!asd
apps:
  app1:
    image: app1
    ports:
      - 5000:5001
`
		invalidYaml2 = `
services:
  postgres:
    image: postgres:1.0
    ports:
      - 8585:8!asd
apps:
  app1:
    image: app1
    ports:
      - 5000:5001
`
	)
	var (
		db          *sql.DB
		sqlxDB      *sqlx.DB
		mock        sqlmock.Sqlmock
		err         error
		clusterName = "MyCustomApps"
		clientset   *fake.Clientset
		username    = "user"
		namespace   = "mystack-user"
		ports       = []int{5000, 5002}
		portMaps    = []*PortMap{
			&PortMap{Port: 5000, TargetPort: 5000},
			&PortMap{Port: 5001, TargetPort: 5002},
		}
		labelMap    = labels.Set{"mystack/routable": "true"}
		listOptions = v1.ListOptions{
			LabelSelector: labelMap.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		}
	)

	mockCluster := func(period, timeout int, username string) *Cluster {
		namespace := fmt.Sprintf("mystack-%s", username)
		return &Cluster{
			Username:  username,
			Namespace: namespace,
			AppDeployments: []*Deployment{
				NewDeployment("test1", username, "app1", ports, nil, nil),
				NewDeployment("test2", username, "app2", ports, nil, nil),
				NewDeployment("test3", username, "app3", ports, []*EnvVar{
					&EnvVar{Name: "VARIABLE_1", Value: "100"},
				}, nil),
			},
			SvcDeployments: []*Deployment{
				NewDeployment(
					"test0",
					username,
					"svc1",
					ports,
					nil,
					&Probe{
						Command:        []string{"echo", "ready"},
						TimeoutSeconds: timeout,
						PeriodSeconds:  period,
					},
				),
			},
			AppServices: []*Service{
				NewService("test1", username, portMaps),
				NewService("test2", username, portMaps),
				NewService("test3", username, portMaps),
			},
			SvcServices: []*Service{
				NewService("test0", username, portMaps),
			},
			Job: NewJob(
				username,
				&Setup{
					Image:          "setup-img",
					PeriodSeconds:  period,
					TimeoutSeconds: timeout,
				},
				[]*EnvVar{
					&EnvVar{Name: "VARIABLE_1", Value: "100"},
				},
			),
			DeploymentReadiness: &mTest.MockReadiness{},
			JobReadiness:        &mTest.MockReadiness{},
		}
	}

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
	})

	Describe("NewCluster", func() {
		BeforeEach(func() {
			db, mock, err = sqlmock.New()
			Expect(err).NotTo(HaveOccurred())
			sqlxDB = sqlx.NewDb(db, "postgres")
		})

		AfterEach(func() {
			err = mock.ExpectationsWereMet()
			Expect(err).NotTo(HaveOccurred())
			db.Close()
		})

		It("should return cluster from config on DB", func() {
			mockedCluster := mockCluster(0, 0, username)

			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))

			cluster, err := NewCluster(sqlxDB, username, clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{})
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.AppDeployments).To(ConsistOf(mockedCluster.AppDeployments))
			Expect(cluster.SvcDeployments).To(ConsistOf(mockedCluster.SvcDeployments))
			Expect(cluster.SvcServices).To(ConsistOf(mockedCluster.SvcServices))
			Expect(cluster.AppServices).To(ConsistOf(mockedCluster.AppServices))
			Expect(cluster.Job).To(Equal(mockedCluster.Job))
		})

		It("should return cluster with non default times from DB", func() {
			mockedCluster := mockCluster(10, 180, username)

			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml2))

			cluster, err := NewCluster(sqlxDB, username, clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{})
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.AppDeployments).To(ConsistOf(mockedCluster.AppDeployments))
			Expect(cluster.SvcDeployments).To(ConsistOf(mockedCluster.SvcDeployments))
			Expect(cluster.SvcServices).To(ConsistOf(mockedCluster.SvcServices))
			Expect(cluster.AppServices).To(ConsistOf(mockedCluster.AppServices))
			Expect(cluster.Job).To(Equal(mockedCluster.Job))
		})

		It("should return error if clusterName doesn't exists on DB", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}))

			cluster, err := NewCluster(sqlxDB, username, clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{})
			Expect(cluster).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("sql: no rows in result set"))
		})

		It("should return error if empty clusterName", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}))

			cluster, err := NewCluster(sqlxDB, username, clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{})
			Expect(cluster).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("sql: no rows in result set"))
		})

		It("should return error with invalid yaml", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(invalidYaml1))

			_, err := NewCluster(sqlxDB, username, clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{})
			Expect(fmt.Sprintf("%T", err)).To(Equal("*errors.YamlError"))
			Expect(err.Error()).To(Equal("strconv.Atoi: parsing \"8!asd\": invalid syntax"))
		})

		It("should return error with invalid yaml 2", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(invalidYaml2))

			_, err := NewCluster(sqlxDB, username, clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{})
			Expect(fmt.Sprintf("%T", err)).To(Equal("*errors.YamlError"))
			Expect(err.Error()).To(Equal("strconv.Atoi: parsing \"8!asd\": invalid syntax"))
		})
	})

	Describe("Create", func() {
		It("should create cluster", func() {
			cluster := mockCluster(0, 0, username)
			err := cluster.Create(nil, clientset)
			Expect(err).NotTo(HaveOccurred())

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(4))

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(4))

			k8sJob, err := clientset.BatchV1().Jobs(namespace).Get("setup")
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sJob).NotTo(BeNil())
			Expect(k8sJob.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(k8sJob.ObjectMeta.Name).To(Equal("setup"))
			Expect(k8sJob.ObjectMeta.Labels["mystack/owner"]).To(Equal(username))
			Expect(k8sJob.ObjectMeta.Labels["app"]).To(Equal("setup"))
			Expect(k8sJob.ObjectMeta.Labels["heritage"]).To(Equal("mystack"))
		})

		It("should return error if creating same cluster twice", func() {
			cluster := mockCluster(0, 0, username)
			err := cluster.Create(nil, clientset)
			Expect(err).NotTo(HaveOccurred())

			err = cluster.Create(nil, clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("namespace for user 'user' already exists"))
		})

		It("should run without setup image", func() {
			cluster := mockCluster(0, 0, username)
			cluster.Job = nil
			err := cluster.Create(nil, clientset)
			Expect(err).NotTo(HaveOccurred())

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(4))

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(4))

			jobs, err := clientset.BatchV1().Jobs(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs.Items).To(BeEmpty())
		})
	})

	Describe("Delete", func() {
		It("should delete cluster", func() {
			cluster := mockCluster(0, 0, username)
			err := cluster.Create(nil, clientset)
			Expect(err).NotTo(HaveOccurred())

			err = cluster.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(NamespaceExists(clientset, namespace)).To(BeFalse())

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(BeEmpty())

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(BeEmpty())
		})

		It("should delete only specified cluster", func() {
			cluster1 := mockCluster(0, 0, "user1")
			err := cluster1.Create(nil, clientset)
			Expect(err).NotTo(HaveOccurred())

			cluster2 := mockCluster(0, 0, "user2")
			err = cluster2.Create(nil, clientset)
			Expect(err).NotTo(HaveOccurred())

			err = cluster1.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(NamespaceExists(clientset, "mystack-user1")).To(BeFalse())
			Expect(NamespaceExists(clientset, "mystack-user2")).To(BeTrue())

			deploys, err := clientset.ExtensionsV1beta1().Deployments("mystack-user1").List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(BeEmpty())

			services, err := clientset.CoreV1().Services("mystack-user1").List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(BeEmpty())

			deploys, err = clientset.ExtensionsV1beta1().Deployments("mystack-user2").List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(4))

			services, err = clientset.CoreV1().Services("mystack-user2").List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(4))
		})

		It("should return error when deleting non existing cluster", func() {
			cluster := mockCluster(0, 0, username)

			err = cluster.Delete(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("namespace for user 'user' not found"))
		})
	})

	Describe("Apps", func() {
		It("should return correct apps if cluster is running", func() {
			cluster := mockCluster(0, 0, "user")
			err := cluster.Create(nil, clientset)

			apps, err := cluster.Apps(clientset)

			Expect(err).NotTo(HaveOccurred())
			Expect(apps).To(ConsistOf(
				"test0.mystack-user",
				"test1.mystack-user",
				"test2.mystack-user",
				"test3.mystack-user",
			))
		})

		It("should return error if cluster is not runnig", func() {
			cluster := mockCluster(0, 0, "user")
			_, err := cluster.Apps(clientset)
			Expect(err).To(HaveOccurred())
		})
	})
})
