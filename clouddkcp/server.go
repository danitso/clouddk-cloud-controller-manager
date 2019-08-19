/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package clouddkcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/danitso/terraform-provider-clouddk/clouddk"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	pathAPTAutoConf = "/etc/apt/apt.conf.d/00auto-conf"
)

var (
	aptAutoConf = heredoc.Doc(`
		Dpkg::Options {
			"--force-confdef";
			"--force-confold";
		}
	`)
)

// CloudServer manages a Cloud.dk server.
type CloudServer struct {
	CloudConfiguration *CloudConfiguration
	Information        clouddk.ServerBody
}

// Create creates a new Cloud.dk server.
func (s *CloudServer) Create(locationID string, packageID string, hostname string) error {
	if s.Information.Identifier != "" {
		return errors.New("The server has already been initialized")
	}

	debugCloudAction(rtServers, "Creating cloud server (hostname: %s)", hostname)

	rootPassword := "p" + s.GetRandomPassword(63)

	body := clouddk.ServerCreateBody{
		Hostname:            hostname,
		Label:               hostname,
		InitialRootPassword: rootPassword,
		Package:             packageID,
		Template:            "ubuntu-18.04-x64",
		Location:            locationID,
	}

	reqBody := new(bytes.Buffer)
	err := json.NewEncoder(reqBody).Encode(body)

	if err != nil {
		return err
	}

	res, err := clouddk.DoClientRequest(s.CloudConfiguration.ClientSettings, "POST", "cloudservers", reqBody, []int{200}, 1, 1)

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server (hostname: %s)", hostname)

		return err
	}

	s.Information = clouddk.ServerBody{}
	err = json.NewDecoder(res.Body).Decode(&s.Information)

	if err != nil {
		return err
	}

	if len(s.Information.NetworkInterfaces) == 0 {
		debugCloudAction(rtServers, "Failed to create cloud server due to lack of network interfaces (hostname: %s)", hostname)

		err = fmt.Errorf("No network interfaces were created for cloud server '%s'", s.Information.Identifier)

		s.Destroy()

		return err
	}

	// Wait for the server to become ready by testing SSH connectivity.
	debugCloudAction(rtServers, "Waiting for cloud server to accept SSH connections (hostname: %s)", hostname)

	var sshClient *ssh.Client

	sshConfig := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password(rootPassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	timeDelay := int64(10)
	timeMax := float64(300)
	timeStart := time.Now()
	timeElapsed := timeStart.Sub(timeStart)

	err = nil

	for timeElapsed.Seconds() < timeMax {
		if int64(timeElapsed.Seconds())%timeDelay == 0 {
			sshClient, err = ssh.Dial("tcp", s.Information.NetworkInterfaces[0].IPAddresses[0].Address+":22", sshConfig)

			if err == nil {
				break
			}

			time.Sleep(1 * time.Second)
		}

		time.Sleep(200 * time.Millisecond)

		timeElapsed = time.Now().Sub(timeStart)
	}

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server due to SSH timeout (hostname: %s)", hostname)

		s.Destroy()

		return err
	}

	defer sshClient.Close()

	s.Information.Booted = true

	// Configure the package manager for unattended upgrades.
	debugCloudAction(rtServers, "Creating new SFTP client (hostname: %s)", hostname)

	sftpClient, err := s.SFTP(sshClient)

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server due to SFTP errors (hostname: %s)", hostname)

		s.Destroy()

		return err
	}

	defer sftpClient.Close()

	debugCloudAction(rtServers, "Uploading file to '%s' (hostname: %s)", pathAPTAutoConf, hostname)

	err = s.UploadFile(sftpClient, pathAPTAutoConf, bytes.NewBufferString(aptAutoConf))

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server because file '%s' could not be uploaded (hostname: %s)", pathAPTAutoConf, hostname)

		s.Destroy()

		return err
	}

	// Configure the server by installing the required software and authorizing the SSH key.
	debugCloudAction(rtServers, "Creating new SSH session (hostname: %s)", hostname)

	sshSession, err := sshClient.NewSession()

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server due to SSH session errors (hostname: %s)", hostname)

		s.Destroy()

		return err
	}

	defer sshSession.Close()

	debugCloudAction(rtServers, "Upgrading and configuring the operating system (hostname: %s)", hostname)

	_, err = sshSession.CombinedOutput(
		fmt.Sprintf("echo '%s' >> ~/.ssh/authorized_keys && ", strings.TrimSpace(s.CloudConfiguration.PublicKey)) +
			"sed -i 's/#\\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config && " +
			"systemctl restart ssh && " +
			"swapoff -a && " +
			"sed -i '/ swap / s/^/#/' /etc/fstab && " +
			"sed -i 's/us.archive.ubuntu.com/mirrors.dotsrc.org/' /etc/apt/sources.list && " +
			"export DEBIAN_FRONTEND=noninteractive && " +
			"while fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do sleep 1; done && " +
			"while fuser /var/lib/dpkg/lock >/dev/null 2>&1; do sleep 1; done && " +
			"apt-get -qq update && " +
			"apt-get -qq upgrade -y && " +
			"apt-get -qq dist-upgrade -y && " +
			"apt-get -qq install -y apt-transport-https ca-certificates software-properties-common",
	)

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server due to shell errors (hostname: %s)", hostname)

		s.Destroy()

		return err
	}

	return nil
}

// Destroy destroys a Cloud.dk server.
func (s *CloudServer) Destroy() error {
	if s.Information.Identifier == "" {
		return errors.New("The server has not been initialized")
	}

	debugCloudAction(rtServers, "Destroying cloud server (hostname: %s)", s.Information.Hostname)

	_, err := clouddk.DoClientRequest(
		s.CloudConfiguration.ClientSettings,
		"DELETE",
		fmt.Sprintf("cloudservers/%s", s.Information.Identifier),
		new(bytes.Buffer),
		[]int{200, 404},
		60,
		10,
	)

	if err != nil {
		debugCloudAction(rtServers, "Failed to destroy cloud server (hostname: %s)", s.Information.Hostname)

		return err
	}

	s.Information = clouddk.ServerBody{}

	return nil
}

// GetRandomPassword generates a random password of a fixed length.
func (s *CloudServer) GetRandomPassword(length int) string {
	var b strings.Builder

	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}

	return b.String()
}

// InitializeByHostname initializes a CloudServer based on a hostname.
func (s *CloudServer) InitializeByHostname(hostname string) (notFound bool, e error) {
	if s.Information.Identifier != "" {
		return false, errors.New("The server has already been initialized")
	}

	if hostname == "" {
		return false, errors.New("Cannot retrieve a server without a hostname")
	}

	res, err := clouddk.DoClientRequest(
		s.CloudConfiguration.ClientSettings,
		"GET",
		fmt.Sprintf("cloudservers?hostname=%s", url.QueryEscape(hostname)),
		new(bytes.Buffer),
		[]int{200},
		1,
		1,
	)

	if err != nil {
		return false, err
	}

	servers := make(clouddk.ServerListBody, 0)
	err = json.NewDecoder(res.Body).Decode(&servers)

	if err != nil {
		return false, err
	}

	for _, v := range servers {
		if v.Hostname == hostname {
			s.Information = v

			return false, nil
		}
	}

	return true, fmt.Errorf("Failed to retrieve the server object for hostname '%s'", hostname)
}

// InitializeByID initializes a CloudServer based on an identifier.
func (s *CloudServer) InitializeByID(id string) (notFound bool, e error) {
	if s.Information.Identifier != "" {
		return false, errors.New("The server has already been initialized")
	}

	if id == "" {
		return false, errors.New("Cannot retrieve a server without an identifier")
	}

	res, err := clouddk.DoClientRequest(
		s.CloudConfiguration.ClientSettings,
		"GET",
		fmt.Sprintf("cloudservers/%s", id),
		new(bytes.Buffer),
		[]int{200},
		1,
		1,
	)

	if err != nil {
		return (res.StatusCode == 404), err
	}

	err = json.NewDecoder(res.Body).Decode(&s.Information)

	if err != nil {
		return false, err
	}

	return false, nil
}

// SFTP creates a new SFTP client for a Cloud.dk server.
func (s *CloudServer) SFTP(sshClient *ssh.Client) (*sftp.Client, error) {
	var err error

	newSSHClient := sshClient

	if newSSHClient == nil {
		newSSHClient, err = s.SSH()

		if err != nil {
			return nil, err
		}
	}

	sftpClient, err := sftp.NewClient(newSSHClient)

	if err != nil {
		return nil, err
	}

	return sftpClient, nil
}

// SSH establishes a new SSH connection to a Cloud.dk server.
func (s *CloudServer) SSH() (*ssh.Client, error) {
	if s.Information.Identifier == "" {
		return nil, errors.New("The server has not been initialized")
	}

	sshPrivateKeyBuffer := bytes.NewBufferString(s.CloudConfiguration.PrivateKey)
	sshPrivateKeySigner, err := ssh.ParsePrivateKey(sshPrivateKeyBuffer.Bytes())

	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(sshPrivateKeySigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshClient, err := ssh.Dial("tcp", s.Information.NetworkInterfaces[0].IPAddresses[0].Address+":22", sshConfig)

	if err != nil {
		return nil, err
	}

	return sshClient, nil
}

// UploadFile uploads a file to the server.
func (s *CloudServer) UploadFile(sftpClient *sftp.Client, filePath string, fileContents *bytes.Buffer) error {
	newSFTPClient := sftpClient

	if newSFTPClient == nil {
		sshClient, err := s.SSH()

		if err != nil {
			return err
		}

		defer sshClient.Close()

		newSFTPClient, err = s.SFTP(sshClient)

		if err != nil {
			return err
		}

		defer newSFTPClient.Close()
	}

	dir := filepath.Dir(filePath)
	err := newSFTPClient.MkdirAll(dir)

	if err != nil {
		return err
	}

	remoteFile, err := newSFTPClient.Create(filePath)

	if err != nil {
		return err
	}

	defer remoteFile.Close()

	_, err = remoteFile.ReadFrom(fileContents)

	if err != nil {
		return err
	}

	return nil
}
