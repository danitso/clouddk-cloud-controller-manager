/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"

	_ "k8s.io/kubernetes/pkg/util/prometheusclientgo"
	_ "k8s.io/kubernetes/pkg/version/prometheus"

	"github.com/danitso/clouddk-cloud-controller-manager/clouddkcp"
)

// main initializes the cloud provider.
func main() {
	rand.Seed(time.Now().UnixNano())

	command := app.NewCloudControllerManagerCommand()
	command.Flags().VisitAll(func(fl *pflag.Flag) {
		var err error

		switch fl.Name {
		case "allow-untagged-cloud", "authentication-skip-lookup":
			err = fl.Value.Set("true")
		case "cloud-provider":
			err = fl.Value.Set(clouddkcp.ProviderName)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to set flag %q: %s\n", fl.Name, err)
			os.Exit(1)
		}
	})

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
