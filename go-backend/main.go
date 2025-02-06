package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

// CloudflareConfig holds the configuration for Cloudflare API
type CloudflareConfig struct {
	AccountID string
	APIToken  string
	BaseURL   string
}

// VideoStatus represents the status of a video
type VideoStatus struct {
	State           string `json:"state"`
	ErrorReasonCode string `json:"errorReasonCode"`
	ErrorReasonText string `json:"errorReasonText"`
}

// CloudflareResult represents the result field in Cloudflare's response
type CloudflareResult struct {
	UID           string      `json:"uid"`
	Preview       string      `json:"preview"`
	Status        VideoStatus `json:"status"`
	ReadyToStream bool        `json:"readyToStream"`
	Thumbnail     string      `json:"thumbnail"`
	Playback      struct {
		HLS  string `json:"hls"`
		Dash string `json:"dash"`
	} `json:"playback"`
	Meta struct {
		Name string `json:"name"`
	} `json:"meta"`
}

// VideoUploadResponse represents the complete response from Cloudflare
type VideoUploadResponse struct {
	Result   CloudflareResult `json:"result"`
	Success  bool             `json:"success"`
	Errors   interface{}      `json:"errors"`
	Messages []string         `json:"messages"`
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Println("Error loading .env file")
	}

	// Initialize configuration
	config := CloudflareConfig{
		AccountID: os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
		APIToken:  os.Getenv("CLOUDFLARE_API_TOKEN"),
		BaseURL:   os.Getenv("CLOUDFLARE_BASE_URL"),
	}

	// Create new Fiber app
	app := fiber.New()

	// Enable CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173", // Vite default port
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST",
	}))

	// Upload endpoint
	app.Post("/api/upload", func(c *fiber.Ctx) error {
		fmt.Printf("Using Account ID: %s\n", config.AccountID)
		fmt.Printf("Base URL: %s\n", config.BaseURL)

		// Get file from request
		file, err := c.FormFile("video")
		if err != nil {
			fmt.Printf("Form file error: %v\n", err)
			return c.Status(400).JSON(fiber.Map{
				"error":   "No video file provided",
				"details": err.Error(),
			})
		}

		fmt.Printf("Received file: %s, size: %d\n", file.Filename, file.Size)

		// Open the file
		fileContent, err := file.Open()
		if err != nil {
			fmt.Printf("File open error: %v\n", err)
			return c.Status(500).JSON(fiber.Map{
				"error":   "Could not open file",
				"details": err.Error(),
			})
		}
		defer fileContent.Close()

		// Create multipart form data
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", file.Filename)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Could not create form file",
				"details": err.Error(),
			})
		}

		// Copy file content to form
		if _, err := io.Copy(part, fileContent); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Could not copy file content",
				"details": err.Error(),
			})
		}
		writer.Close()

		// Create Cloudflare Stream upload request
		url := fmt.Sprintf("%s/accounts/%s/stream", config.BaseURL, config.AccountID)
		fmt.Printf("Making request to: %s\n", url)

		req, err := http.NewRequest("POST", url, body)
		if err != nil {
			fmt.Printf("Request creation error: %v\n", err)
			return c.Status(500).JSON(fiber.Map{
				"error":   "Could not create request",
				"details": err.Error(),
			})
		}

		// Set headers
		req.Header.Set("Authorization", "Bearer "+config.APIToken)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Send request to Cloudflare
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Cloudflare request error: %v\n", err)
			return c.Status(500).JSON(fiber.Map{
				"error":   "Failed to upload to Cloudflare",
				"details": err.Error(),
			})
		}
		defer resp.Body.Close()

		// Read response body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error reading response body: %v\n", err)
			return c.Status(500).JSON(fiber.Map{
				"error":   "Could not read response",
				"details": err.Error(),
			})
		}

		fmt.Printf("Cloudflare Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Cloudflare Response Body: %s\n", string(bodyBytes))

		// Parse response
		var result VideoUploadResponse
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			fmt.Printf("JSON parse error: %v\n", err)
			return c.Status(500).JSON(fiber.Map{
				"error":    "Could not parse response",
				"details":  err.Error(),
				"response": string(bodyBytes),
			})
		}

		// Check if upload was successful
		if !result.Success {
			return c.Status(400).JSON(fiber.Map{
				"error":    "Upload failed",
				"details":  result.Errors,
				"response": string(bodyBytes),
			})
		}

		return c.JSON(result)
	})

	// Get video status endpoint
	app.Get("/api/video/:uid", func(c *fiber.Ctx) error {
		uid := c.Params("uid")
		url := fmt.Sprintf("%s/accounts/%s/stream/%s", config.BaseURL, config.AccountID, uid)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Could not create request",
				"details": err.Error(),
			})
		}

		req.Header.Set("Authorization", "Bearer "+config.APIToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Failed to get video status",
				"details": err.Error(),
			})
		}
		defer resp.Body.Close()

		var result VideoUploadResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Could not parse response",
				"details": err.Error(),
			})
		}

		return c.JSON(result)
	})

	// Start server
	fmt.Println("Server starting on port 3000...")
	app.Listen(":3000")
}
