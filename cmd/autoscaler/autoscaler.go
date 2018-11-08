package autoscaler

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
)

// Options holds settable options for LB Autoscaler.
type Options struct {
	UpscaleConsecutive   int
	DownscaleConsecutive int
	Deployment           string
	MinScale             int
	MaxScale             int
	MinFreeSlots         int
	MaxFreeSlots         int
	Cooldown             time.Duration
}

// DefaultOptions holds default options values for LB Autoscaler.
var DefaultOptions = Options{
	UpscaleConsecutive:   2,
	DownscaleConsecutive: 5,
	MinScale:             1,
	MaxScale:             8,
	MinFreeSlots:         6,
	MaxFreeSlots:         16,
	Cooldown:             10 * time.Minute,
}

// Autoscaler defines a Load Balancer Autoscaler.
type Autoscaler struct {
	client  *http.Client
	url     string
	token   string
	options Options
}

const (
	checkPeriod = 10 * time.Second
)

// New initializes new Load Balancer Autoscaler.
func New(options Options) (*Autoscaler, error) {
	// Create the in-cluster config.
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, errors.New("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	}
	// Load k8s files.
	namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return nil, err
	}
	ca, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, err
	}
	token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, err
	}

	// Configure TLS.
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	tlsConfig.RootCAs = x509.NewCertPool()
	if !tlsConfig.RootCAs.AppendCertsFromPEM(ca) {
		return nil, errors.New("certificate authority doesn't contain any certificates")
	}

	// Configure http transport to use.
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	if err = http2.ConfigureTransport(transport); err != nil {
		return nil, err
	}

	// Create deployment scale url.
	u, err := url.Parse("https://" + host + ":" + port)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "apis/extensions/v1beta1/namespaces", string(namespace), "deployments", options.Deployment, "scale")

	return &Autoscaler{
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
		url:     u.String(),
		token:   string(token),
		options: options,
	}, nil
}

type scaleResponseSpec struct {
	Spec   replicaSpec
	Status replicaSpec
}

type replicaSpec struct {
	Replicas int
}

func (a *Autoscaler) getReplicas() (*scaleResponseSpec, error) {
	req, err := http.NewRequest("GET", a.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	res, err := a.client.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	respBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var respSpec scaleResponseSpec
	err = json.Unmarshal(respBody, &respSpec)
	if err != nil {
		return nil, err
	}

	return &respSpec, nil
}

func (a *Autoscaler) updateReplicas(updated int) error {
	jsonData := fmt.Sprintf(`{"spec": {"replicas": %d}}`, updated)
	req, err := http.NewRequest("PATCH", a.url, bytes.NewBuffer([]byte(jsonData)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", "application/merge-patch+json")
	res, err := a.client.Do(req)
	if err != nil {
		return err
	}
	return res.Body.Close()
}

// Start runs autoscaler loop in background and returns channel that if closed will stop the loop.
func (a *Autoscaler) Start() chan struct{} { // nolint: gocyclo
	stop := make(chan struct{})
	ticker := time.NewTicker(checkPeriod)
	freeSlotsCounter := expvar.Get("slots").(*expvar.Int)
	workersCounter := expvar.Get("workers").(*expvar.Int)
	upscaleConsecutive := 0
	downscaleConsecutive := 0

	go func() {
		lastUpdate := time.Now()

		for {
			select {
			case <-ticker.C:
				workerCount := int(workersCounter.Value())
				// Do not scale if no workers are connected.
				if workerCount <= 0 {
					continue
				}

				freeSlots := int(freeSlotsCounter.Value())
				replicaSpec, err := a.getReplicas()
				if err != nil {
					logrus.WithField("options", a.options).WithError(err).Warn("Autoscaling.GetScale failed")
					continue
				}
				changed := false
				desiredReplicas, curReplicas := replicaSpec.Spec.Replicas, replicaSpec.Status.Replicas
				switch {
				case workerCount == curReplicas && freeSlots < a.options.MinFreeSlots && curReplicas == desiredReplicas:
					if desiredReplicas < a.options.MaxScale {
						if upscaleConsecutive+1 > a.options.UpscaleConsecutive {
							desiredReplicas++
							changed = true
						} else {
							upscaleConsecutive++
						}
					}

				case freeSlots > a.options.MaxFreeSlots && time.Since(lastUpdate) > a.options.Cooldown:

					if desiredReplicas > a.options.MinScale {
						if downscaleConsecutive+1 > a.options.DownscaleConsecutive {
							desiredReplicas--
							changed = true
						} else {
							downscaleConsecutive++
						}
					}

				default:
					downscaleConsecutive = 0
					upscaleConsecutive = 0
				}

				if changed {
					logrus.WithFields(logrus.Fields{"current": curReplicas, "desired": desiredReplicas}).Info("Autoscaling.UpdateScale")

					lastUpdate = time.Now()
					err = a.updateReplicas(desiredReplicas)
					if err != nil {
						logrus.WithError(err).WithField("replicas", desiredReplicas).Warn("Autoscaling.UpdateScale failed")

					}
				}

			case <-stop:
				return
			}
		}
	}()

	return stop
}
