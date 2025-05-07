package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/Hayao0819/Kamisato/conf"
	"github.com/gin-gonic/gin"
)

var config conf.Config

func main() {
	var err error
	config, err = conf.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello, Gin!")
	})

	// Blinky API endpoints
	router.POST("/login", loginHandler)
	router.POST("/logout", logoutHandler)
	router.POST("/upload", uploadHandler)
	router.POST("/remove", removeHandler)

	log.Printf("Listening on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func loginHandler(c *gin.Context) {
	username, password, ok := c.Request.BasicAuth()
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if username == config.Username && password == config.Password {
		c.String(http.StatusOK, "Login successful")
		return
	}

	c.AbortWithStatus(http.StatusUnauthorized)
}

func logoutHandler(c *gin.Context) {
	c.String(http.StatusOK, "Logout successful")
}

func uploadHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}

	filename := file.Filename
	log.Println(filename)

	// Save the file to the repository
	filePath := config.RepoPath + "/" + filename
	err = c.SaveUploadedFile(file, filePath)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// Add the package to the repository database
	useSignedDB := false
	var gnupgDir *string
	err = RepoAdd(config.DBPath, filePath, useSignedDB, gnupgDir)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("repo-add err: %s", err.Error()))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", filename))
}

// RepoAdd uses `repo-add` to add the *.pkg.tar.zst located at pkgFilePath to the Pacman repository
// database located at dbPath.
//
// If pkgFilePath is an empty string (""), the argument will not be passed to `repo-add`. This is
// useful for creating an empty database
//
// If the database should be signed, set useSignedDB to true and set gnupgDir to the directory
// to store the keyring in.
func RepoAdd(dbPath, pkgFilePath string, useSignedDB bool, gnupgDir *string) error {
	// Build and run repo-add command, including the --sign arg if requested
	args := []string{"-q", "-R", "--nocolor"}
	if useSignedDB {
		args = append(args, "--sign")
	}
	args = append(args, dbPath)

	if pkgFilePath != "" {
		args = append(args, pkgFilePath)
	}

	cmd := exec.Command("repo-add", args...)
	if gnupgDir != nil {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GNUPGHOME=%s", *gnupgDir))
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("RepoAdd: error running %s (output: %s): %w", cmd.String(), string(out), err)
	}

	return nil
}

func removeHandler(c *gin.Context) {
	packageName := c.Query("package")
	if packageName == "" {
		c.String(http.StatusBadRequest, "Package name is required")
		return
	}

	// Remove the package from the repository
	useSignedDB := false
	var gnupgDir *string
	err := RepoRemove(config.DBPath, packageName, useSignedDB, gnupgDir)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("repo-remove err: %s", err.Error()))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("'%s' removed!", packageName))
}

// RepoRemove uses `repo-remove` to remove the specified package from the database located at
// dbPath.
//
// If the database should be signed, set useSignedDB to true and set gnupgDir to the directory
// to store the keyring in.
func RepoRemove(dbPath, packageName string, useSignedDB bool, gnupgDir *string) error {
	// Build and run repo-add command, including the --sign arg if requested
	args := []string{"-q", "--nocolor"}
	if useSignedDB {
		args = append(args, "--sign")
	}
	args = append(args, dbPath, packageName)

	cmd := exec.Command("repo-remove", args...)
	if gnupgDir != nil {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GNUPGHOME=%s", *gnupgDir))
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("RepoRemove: error running %s (output: %s): %w", cmd.String(), string(out), err)
	}

	return nil
}
