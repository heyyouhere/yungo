package main

import (
	"bufio"
	"syscall"
	"os/signal"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Status struct{
    Id string
    Names []string
    Created int
    State string
    Status string
	Image string
	Command string

}

func clearScreen() {
    fmt.Printf("\033[2J\033[H");
}

func (s *Status) Display(hideRunning bool, showStopped bool) string{
	var sb strings.Builder
	const (
		red   = "\033[31m" // Red color
		green = "\033[32m" // Green color
		circle = "●"
		reset = "\033[0m"  // Reset to default color
	)
	if s.State == "running"{
		if hideRunning{
			return ""
		}
		sb.WriteString(green)
	} else {
		if !showStopped{
			return ""
		}
		sb.WriteString(red)
	}
	sb.WriteString(circle)
	sb.WriteString(reset)
	sb.WriteString(fmt.Sprintf(" %s\t", s.Names[0]))
	sb.WriteString(fmt.Sprintf("%s\t", s.Status))
	sb.WriteString(fmt.Sprintf("%s\t", s.Image))
	sb.WriteString("\n")
	return sb.String()
}


type Host struct{
	HostName string
	Port string
	User string
	IdentityFile string
}

func ParseSSHConfig(filePath string, privateKeyPath string, hosts *[]Host) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var currentHost *Host
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 2{
			switch parts[0] {
			case "Host":
				if currentHost != nil{
					if len(currentHost.IdentityFile) == 0{
						currentHost.IdentityFile = privateKeyPath
					}
					*hosts = append(*hosts, *currentHost)
				}
				currentHost = &Host{}
			case "HostName":
				currentHost.HostName = parts[1]
			case "User":
				currentHost.User = parts[1]
			case "Port":
				currentHost.Port = parts[1]
			case "IdentityFile":
				currentHost.IdentityFile = parts[1]
			}
		} else {
			return fmt.Errorf("Unexpected line in %s", filePath)
		}
	}

	if currentHost != nil{
		if len(currentHost.IdentityFile) == 0{
			currentHost.IdentityFile = privateKeyPath
		}
		*hosts = append(*hosts, *currentHost)
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// TODO: should statuses be part of dock?
type Dock struct{
	Host Host
	Client *ssh.Client
}


func CreateDock(host Host) (*Dock, error){
	key, err := os.ReadFile(host.IdentityFile)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: host.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	address := fmt.Sprintf("%s:%s", host.HostName, host.Port)
	remoteClient, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, err
	}
	return &Dock{host, remoteClient}, nil
}

func (d *Dock) runCommand(command string) string{
	session, err := d.Client.NewSession()
	if err != nil {
		fmt.Printf("failed to create session: %v", err)
	}
	defer session.Close()

	var b []byte
	if b, err = session.CombinedOutput(command); err != nil {
		fmt.Printf("failed to run: %v", err)
	}
	return string(b)
}

func (d *Dock) GetStatus(socketPath string) ([]Status, error){
	remoteConn, err := d.Client.Dial("unix", socketPath)
	if err != nil{
		fmt.Printf("Could not esstablish connection with: %s\n", socketPath)
	}
	defer remoteConn.Close()
	request := "GET /containers/json?all=true HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
	_, err = remoteConn.Write([]byte(request))
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 1024*1024)
	_, err = remoteConn.Read(buf)
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer(buf)

	res, err := http.ReadResponse(bufio.NewReader(buffer), nil)
	if err != nil{
		return nil, err
	}

	var statuses []Status
	bodyBuff := make([]byte, 1024*1024)
	n, _ := res.Body.Read(bodyBuff)
	err = json.Unmarshal(bodyBuff[:n], &statuses)
	if err != nil {
		return nil, err
	}
	return statuses, nil
}

func (d *Dock) GetLogs(remoteClient *ssh.Client, socketPath string, container_id string) error{
	// TODO: this function does not return logs, since the endpoint writes as
	// Content-Type: application/vnd.docker.multiplexed-stream
	// And I dont know how to handle it in golang
	remoteConn, err := remoteClient.Dial("unix", socketPath)
	if err != nil{
		fmt.Printf("Could not esstablish connection with: %s\n", socketPath)
	}
	defer remoteConn.Close()
	request := fmt.Sprintf("GET /containers/%s/logs?stdout=true&tail=40 HTTP/1.1\r\nHost: localhost\r\n\r\n", container_id)
	_, err = remoteConn.Write([]byte(request))
	if err != nil {
		return err
	}
	buf := make([]byte, 1024*1024*1024)
	n, err := remoteConn.Read(buf)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", string(buf[:n]))

	fmt.Printf("Reading chunk-size\n")
	n1, err := remoteConn.Read(buf)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", string(buf[n:n1]))
	os.Exit(0)

	// res, err := http.ReadResponse(bufio.NewReader(buffer), nil)
	// if err != nil{
	// 	return err
	// }
	// bodyBuff := make([]byte, 1024*1024)
	// res.Body.Read(bodyBuff)
	// fmt.Printf("%s\n", string(bodyBuff))
	return nil
}

func Run(docks []*Dock, target string, hideRunning bool, showStopped bool) string{
	remoteSocketPath := "/var/run/docker.sock"
	var sb strings.Builder
	for _, dock := range docks{
		if len(target) > 0{
			if target != dock.Host.HostName{ continue }
		}
		memory := dock.runCommand(`free -hm | awk 'NR==2{printf "%s/%s", $3, $2}'`)
		dock_uptime := dock.runCommand("uptime")[1:]
		sb.WriteString(fmt.Sprintf("Dock %s@%s [%s]\n%s", dock.Host.User, dock.Host.HostName, memory, dock_uptime))
		statuses, err := dock.GetStatus(remoteSocketPath)
		if err != nil{
			sb.WriteString(fmt.Sprintf("Could not GetStatus of Dock %s, %s\n", dock.Host.HostName, err))
			continue
		}
		sort.Slice(statuses, func(i, j int) bool {
			if statuses[i].State == "running" && statuses[j].State == "running"{
				return len(statuses[i].Names[0]) < len(statuses[j].Names[0])
			}
			if statuses[i].State == "running"{
				return true
			}
			if statuses[j].State == "running"{
				return false
			}
			return false
		})
		for _, s := range statuses{
			sb.WriteString(fmt.Sprintf("%s", s.Display(hideRunning, showStopped)))
			// dock.GetLogs(client, remoteSocketPath, s.Id)
		}
		sb.WriteString("--------------------------------------------------\n")
	}
	return sb.String()
}


func main() {
	home := os.Getenv("HOME")
	var (
		hideRunning = flag.Bool("r", false, "Display running containers")
		showStopped = flag.Bool("s", false, "Display stopped containers")
		help = flag.Bool("help", false, "Displa+1y this message")
		privateKey = flag.String("k", fmt.Sprintf("%s/.ssh/id_rsa", home), "Path to private key")
		hostPath = flag.String("h", fmt.Sprintf("%s/.ssh/config", home), "Path to config file")
		watch = flag.Uint("watch", 0, "Watch mode, secods between updates")
	)
	var target string
	flag.StringVar(&target, "t",  "", "Show only needed targets container")
	flag.StringVar(&target, "target",  "", "Show only needed targets container")

	flag.Parse()
	if *help{
		flag.Usage()
		os.Exit(1)
	}
	privateKeyPath := *privateKey
	if len(privateKeyPath) == 0{
		fmt.Printf("No private key path provided.\n")
		flag.Usage()
		os.Exit(1)
	}
	hosts := []Host{}
	err := ParseSSHConfig(*hostPath, privateKeyPath, &hosts)

	if err != nil{
		fmt.Printf("Could not parse %s, because of %s\n", *hostPath, err)
		os.Exit(1)
	}
	fmt.Printf("Parsed hosted file: %d\n", len(hosts))
	for index, host := range hosts{
		fmt.Printf("   %d. %s@%s\n", index+1, host.User, host.HostName)
	}


	var docks []*Dock
	fmt.Printf("Connecting to %d servers...\n", len(hosts))
	for _, host := range hosts{
		dock, err := CreateDock(host)
		if err != nil {
			fmt.Printf("Could not connect to %s, %s", host.HostName, err)
		}
		docks = append(docks, dock)
	}

	fmt.Printf("Collecting data...\n")
	if *watch > 0{
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func(){
			for {
				sleepTime := *watch
				result := Run(docks, target, *hideRunning, *showStopped)
				clearScreen()
				fmt.Printf("Watch %d sec\n", sleepTime)
				fmt.Printf("%s", result)
				fmt.Printf("<C-c> to exit.")
				time.Sleep(time.Second * time.Duration(sleepTime))
			}}()
			sig := <-sigChan
			// TODO: there is a bug when exiting that nil dereference somewhere
			fmt.Printf("\nReceived signal: %s. Shutting down...\n", sig)
			for _, dock := range docks{
				dock.Client.Close()
			}
			fmt.Println("Exiting program.")
	} else {
		result := Run(docks, target, *hideRunning, *showStopped)
		clearScreen()
		fmt.Printf("%s", result)
		for _, dock := range docks{
			dock.Client.Close()
		}
	}

}
