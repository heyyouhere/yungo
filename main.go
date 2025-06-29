package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

type Dock struct{
	Host string
	Port string
	Username string
}


func (d *Dock) Connect(privateKeyPath string) (*ssh.Client, error){
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{ User: d.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	address := fmt.Sprintf("%s:%s", d.Host, d.Port)
	remoteClient, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, err
	}
	return remoteClient, err
}


func (d *Dock) GetStatus(remoteClient *ssh.Client, socketPath string) ([]byte, error){
	// TODO: this is going to be run in a timeout loop,
	// so it is probably better idea to store remoteConn,
	// rather than remoteClient
	remoteConn, err := remoteClient.Dial("unix", socketPath)
	defer remoteConn.Close()
	request := "GET /containers/json HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
	_, err = remoteConn.Write([]byte(request))
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 1024*1024*1024)
	n, err := remoteConn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}


func main() {
	if len(os.Args) < 2{
		fmt.Printf("No private key path provided.\n")
		fmt.Printf("Usage:\n%s [path_to_private_key]\n", os.Args[0])
		os.Exit(1)
	}
	privateKeyPath := os.Args[1]
	hostsFile, err := os.Open("hosts")
	if err != nil{
		fmt.Printf("Could not open hosts file\n")
	}
	hostsBytes, err := io.ReadAll(hostsFile)
	if err != nil{
		fmt.Printf("Could not read bytes from hosts file\n")
	}
	docks := []Dock{}
	err = json.Unmarshal(hostsBytes, &docks)
	if err != nil{
		fmt.Printf("Could not unmarshal, %s", err)
	}

	remoteSocketPath := "/var/run/docker.sock"
	for _, dock := range docks{
		fmt.Printf("%s@%s -p %s\n", dock.Username, dock.Host, dock.Port)
		client, err := dock.Connect(privateKeyPath)
		if err != nil{
			fmt.Printf("Could not connect to dock %s, %s", dock.Host, err)
			continue
		}
		buff, err := dock.GetStatus(client, remoteSocketPath)
		if err != nil{
			fmt.Printf("Could not GetStatus of Dock %s, %s", dock.Host, err)
			continue
		}
		fmt.Printf("Status %s", buff)
	}
}
