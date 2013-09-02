package main

import (
	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/goauth2/oauth"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func getPrivateKey(filename string) []byte {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Panic("Failed to open private key file: " + err.Error())
	}
	return content
}

type keychain struct {
	key *rsa.PrivateKey
}

func (k *keychain) Key(i int) (interface{}, error) {
	if i != 0 {
		return nil, nil
	}
	return &k.key.PublicKey, nil
}

func (k *keychain) Sign(i int, rand io.Reader, data []byte) (sig []byte, err error) {
	hashFunc := crypto.SHA1
	h := hashFunc.New()
	h.Write(data)
	digest := h.Sum(nil)
	return rsa.SignPKCS1v15(rand, k.key, hashFunc, digest)
}

//  Will return the latest commit hash, waiting on https://github.com/google/go-github/pull/49
func latestGitHubCommit(c *github.Client, repoName string) *github.Repository {
	repo, _, err := c.Repositories.Get("gengo", repoName)
	if err != nil {
		log.Panic(err)
	}
	return repo
}

func remoteCmdOutput(username, hostname, privateKey, cmd string) []byte {
	block, _ := pem.Decode([]byte(privateKey))
	rsakey, _ := x509.ParsePKCS1PrivateKey(block.Bytes)
	clientKey := &keychain{rsakey}
	clientConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.ClientAuth{
			ssh.ClientAuthKeyring(clientKey),
		},
	}
	client, err := ssh.Dial("tcp", hostname, clientConfig)
	if err != nil {
		log.Panic("Failed to dial: " + err.Error())
	}
	session, err := client.NewSession()
	if err != nil {
		log.Panic("Failed to create session: " + err.Error())
	}
	defer session.Close()
	output, err := session.Output(cmd)
	if err != nil {
		log.Panic("Failed to run cmd: " + err.Error())
	}
	return output
}

func latestDeployedCommit(hostname string) []byte {
	// TODO: change this
	privateKey := string(getPrivateKey("path-to-private-key"))
	output := remoteCmdOutput("deployer", hostname, privateKey, "whoami")

	return output
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("templates/index.html")
	t.Execute(w, nil)
}

func main() {
	githubToken := os.Getenv("GITHUB_API_TOKEN")
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: githubToken},
	}
	client := github.NewClient(t.Client())
	fmt.Println(client)
	output := latestDeployedCommit("www-qa-02.gengo.com:22")
    fmt.Println(output)

	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	http.ListenAndServe(":8080", r)
}
