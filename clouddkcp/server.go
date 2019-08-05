package clouddkcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/danitso/terraform-provider-clouddk/clouddk"
	"golang.org/x/crypto/ssh"
)

// CloudServer manages a Cloud.dk server.
type CloudServer struct {
	CloudConfiguration *CloudConfiguration
	Information        clouddk.ServerBody
}

// Create creates a new Cloud.dk server.
func (s CloudServer) Create(locationID string, packageID string, hostname string) error {
	if s.Information.Identifier != "" {
		return errors.New("The server has already been initialized")
	}

	rootPassword := s.GetRandomPassword(64)

	body := clouddk.ServerCreateBody{
		Hostname:            hostname,
		Label:               hostname,
		InitialRootPassword: rootPassword,
		Package:             packageID,
		Template:            "ubuntu-18.04-x64",
		Location:            locationID,
	}

	reqBody := new(bytes.Buffer)
	encodeErr := json.NewEncoder(reqBody).Encode(body)

	if encodeErr != nil {
		return encodeErr
	}

	res, resErr := clouddk.DoClientRequest(s.CloudConfiguration.ClientSettings, "POST", "cloudservers", reqBody, []int{200}, 60, 10)

	if resErr != nil {
		return resErr
	}

	s.Information = clouddk.ServerBody{}
	decodeErr := json.NewDecoder(res.Body).Decode(&s.Information)

	if decodeErr != nil {
		return decodeErr
	}

	// Wait for the server to become ready by testing SSH connectivity.
	var sshClient *ssh.Client
	var sshErr error

	sshConfig := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password(rootPassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	timeDelay := int64(10)
	timeMax := float64(600)
	timeStart := time.Now()
	timeElapsed := timeStart.Sub(timeStart)

	for timeElapsed.Seconds() < timeMax {
		if int64(timeElapsed.Seconds())%timeDelay == 0 {
			sshClient, sshErr = ssh.Dial("tcp", s.Information.NetworkInterfaces[0].IPAddresses[0].Address+":22", sshConfig)

			if sshErr == nil {
				break
			}

			time.Sleep(1 * time.Second)
		}

		time.Sleep(200 * time.Millisecond)

		timeElapsed = time.Now().Sub(timeStart)
	}

	if sshErr != nil {
		s.Destroy()

		return sshErr
	}

	s.Information.Booted = true

	// Configure the server by installing the required software and authorizing the SSH key.
	sshSession, sshSessionErr := sshClient.NewSession()

	if sshSessionErr != nil {
		sshClient.Close()
		s.Destroy()

		return sshSessionErr
	}

	_, sshOuputErr := sshSession.CombinedOutput(
		fmt.Sprintf("echo '%s' >> ~/.ssh/authorized_keys && ", strings.TrimSpace(s.CloudConfiguration.PublicKey)) +
			"sed -i 's/#PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config && " +
			"systemctl restart ssh",
	)

	if sshOuputErr != nil {
		sshClient.Close()
		s.Destroy()

		return sshSessionErr
	}

	sshClient.Close()

	return nil
}

// Destroy destroys a Cloud.dk server.
func (s CloudServer) Destroy() error {
	if s.Information.Identifier == "" {
		return errors.New("The server has not been initialized")
	}

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
		return err
	}

	s.Information = clouddk.ServerBody{}

	return nil
}

// GetRandomPassword generates a random password of a fixed length.
func (s CloudServer) GetRandomPassword(length int) string {
	var b strings.Builder

	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZÅÄÖabcdefghijklmnopqrstuvwxyzåäö0123456789")

	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}

	return b.String()
}

// InitializeByHostname initializes a CloudServer based on a hostname.
func (s CloudServer) InitializeByHostname(hostname string) (notFound bool, e error) {
	if s.Information.Identifier != "" {
		return false, errors.New("The server has already been initialized")
	}

	res, resErr := clouddk.DoClientRequest(
		s.CloudConfiguration.ClientSettings,
		"GET",
		fmt.Sprintf("cloudservers?hostname=%s", hostname),
		new(bytes.Buffer),
		[]int{200},
		1,
		1,
	)

	if resErr != nil {
		return false, resErr
	}

	servers := make(clouddk.ServerListBody, 0)
	decodeErr := json.NewDecoder(res.Body).Decode(&servers)

	if decodeErr != nil {
		return false, decodeErr
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
func (s CloudServer) InitializeByID(id string) (notFound bool, e error) {
	if s.Information.Identifier != "" {
		return true, errors.New("The server has already been initialized")
	}

	res, resErr := clouddk.DoClientRequest(
		s.CloudConfiguration.ClientSettings,
		"GET",
		fmt.Sprintf("cloudservers/%s", id),
		new(bytes.Buffer),
		[]int{200},
		1,
		1,
	)

	if resErr != nil {
		return (res.StatusCode == 404), resErr
	}

	decodeErr := json.NewDecoder(res.Body).Decode(&s.Information)

	if decodeErr != nil {
		return false, decodeErr
	}

	return false, nil
}

// SSH establishes a new SSH connection to a Cloud.dk server.
func (s CloudServer) SSH() (*ssh.Client, error) {
	if s.Information.Identifier == "" {
		return nil, errors.New("The server has not been initialized")
	}

	sshPrivateKeyBuffer := bytes.NewBufferString(s.CloudConfiguration.PrivateKey)
	sshPrivateKeySigner, sshPrivateKeyErr := ssh.ParsePrivateKey(sshPrivateKeyBuffer.Bytes())

	if sshPrivateKeyErr != nil {
		return nil, sshPrivateKeyErr
	}

	sshConfig := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(sshPrivateKeySigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshClient, sshErr := ssh.Dial("tcp", s.Information.NetworkInterfaces[0].IPAddresses[0].Address+":22", sshConfig)

	if sshErr != nil {
		return nil, sshErr
	}

	return sshClient, nil
}
