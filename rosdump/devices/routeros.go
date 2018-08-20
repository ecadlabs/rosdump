package devices

/*
import (
	"crypto/tls"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/routeros.v2"
)

type RouterOS struct {
	utils.SSHConfig
	Host     string
	APIPort  string
	SSHPort  string
	Username string
	Password string
	Binary   bool
	Verbose  bool
}

type routerosReply struct {
	io.ReadCloser
	client *ssh.Client
}

func (r *routerosReply) Close() error {
	if err := r.ReadCloser.Close(); err != nil {
		return err
	}
	return r.client.Close()
}

func (r *RouterOS) Export() (response io.ReadCloser, err error) {
	tlsConfig := tls.Config{
		InsecureSkipVerify: true,
	}

	client, err := routeros.DialTLS(net.JoinHostPort(r.Host, r.APIPort), r.Username, r.Password, &tlsConfig)
	if err != nil {
		return nil, err
	}

	defer client.Close()

	prefix := "backup_" + time.Now().UTC().Format(time.RFC3339)

	var (
		fileName string
		cmd      []string
	)

	if r.Binary {
		cmd = []string{"/system/backup/save", "=name=" + prefix}
		fileName = prefix + ".backup"
	} else {
		cmd = []string{"/export", "=file=" + prefix}
		if r.Verbose {
			cmd = append(cmd, "=verbose=")
		}
		fileName = prefix + ".rsc"
	}

	if _, err := client.Run(cmd...); err != nil {
		return nil, err
	}

	// Wait for file to appear
	var fileId string
	for {
		re, err := client.Run("/file/print", "?name="+fileName)
		if err != nil {
			return nil, err
		}

		if len(re.Re) != 0 {
			fileId = re.Re[0].Map[".id"]
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Download file
	sshClient, err := utils.DialSSH(&r.SSHConfig, net.JoinHostPort(r.Host, r.SSHPort), r.Username, r.Password)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			sshClient.Close()
		}
	}()

	_, data, err := utils.SCPFetch(sshClient, fileName)
	if err != nil {
		return nil, err
	}

	// Remove
	if _, err := client.Run("/file/remove", "=numbers="+fileId); err != nil {
		return nil, err
	}

	res := &routerosReply{
		ReadCloser: data,
		client:     sshClient,
	}

	return res, nil
}

func main() {
	var (
		cfg     RouterOS
		keyFile string
	)

	flag.StringVar(&cfg.Host, "h", "192.168.88.1", "Host")
	flag.StringVar(&cfg.APIPort, "api-port", "8729", "API Port")
	flag.StringVar(&cfg.SSHPort, "ssh-port", "22", "SSH Port")
	flag.StringVar(&cfg.Username, "username", "admin", "Username")
	flag.StringVar(&cfg.Password, "password", "", "Password")
	flag.StringVar(&keyFile, "i", filepath.Join(os.Getenv("HOME"), "/.ssh/id_rsa"), "SSH key file")

	flag.Parse()

	if key, err := ioutil.ReadFile(keyFile); err == nil {
		cfg.PrivateKeyPem = key
	}

	data, err := cfg.Export()
	if err != nil {
		log.Fatal(err)
	}

	if err := data.Close(); err != nil {
		log.Fatal(err)
	}

	if err != nil {
		log.Fatal(err)
	}
}

var _ Exporter = &RouterOS{}
*/
