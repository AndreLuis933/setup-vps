package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	webhookDir      = "/app/webhook_jobs"
	webhookSecret   string
	repoNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

type GitHubWebhookPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name string `json:"name"`
	} `json:"repository"`
}

func init() {
	webhookSecret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	webhookSecret = strings.Trim(webhookSecret, "' \t\n\r")
	if webhookSecret == "" {
		panic("GITHUB_WEBHOOK_SECRET environment variable is required")
	}

	if err := os.MkdirAll(webhookDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create webhook directory: %v", err))
	}
}

func main() {
	router := gin.New()

	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	}))

	router.Use(gin.Recovery())
	router.SetTrustedProxies([]string{
		"10.11.12.0/24",
	})

	router.HEAD("/health", healthCheckHandler)
	router.POST("/webhook/github", githubWebhookHandler)

	router.Run(":8000")
}

func healthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

func verifySignature(body []byte, signature string) error {
	if signature == "" || len(signature) < 7 || signature[:7] != "sha256=" {
		return fmt.Errorf("missing or invalid signature")
	}
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	receivedMAC := signature[7:] // Remove "sha256=" prefix

	if !hmac.Equal([]byte(expectedMAC), []byte(receivedMAC)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

func runDeploy(repository string) {
	repoDir := filepath.Join(webhookDir, repository)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		fmt.Printf("Error creating repository directory: %v\n", err)
		return
	}

	fmt.Printf("Deploy triggered for repository: %s\n", repository)
}

func githubWebhookHandler(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Printf("[WEBHOOK] Failed to read body: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	signature := c.GetHeader("X-Hub-Signature-256")
	if err := verifySignature(body, signature); err != nil {
		fmt.Printf("[WEBHOOK] Signature failed: %v | sig_present: %v | body_len: %d\n",
			err, signature != "", len(body))
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	event := c.GetHeader("X-GitHub-Event")
	if event != "push" {
		fmt.Printf("[WEBHOOK] Ignored event type: %s\n", event)
		c.JSON(http.StatusOK, gin.H{"ok": true, "ignored": "not a push"})
		return
	}

	var payload GitHubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Printf("[WEBHOOK] Invalid JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	if payload.Ref != "refs/heads/main" {
		fmt.Printf("[WEBHOOK] Ignored ref: %s | repo: %s\n", payload.Ref, payload.Repository.Name)
		c.JSON(http.StatusOK, gin.H{"ok": true, "ignored": "not main"})
		return
	}

	repository := payload.Repository.Name
	if !repoNamePattern.MatchString(repository) {
		fmt.Printf("[WEBHOOK] Invalid or empty repo name: %q\n", repository)
		c.JSON(http.StatusOK, gin.H{"ok": true, "ignored": "invalid repository"})
		return
	}

	fmt.Printf("[WEBHOOK] Deploy queued: %s\n", repository)

	go runDeploy(repository)

	c.JSON(http.StatusOK, gin.H{"ok": true, "queued": true})
}
