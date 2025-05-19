package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/earentir/gitearelease"
	"github.com/spf13/cobra"
)

// Config represents the configuration for the application
type Config struct {
	GiteaURL string                 `json:"gitea_url"`
	Repos    map[string]RepoDetails `json:"repos"`
}

// RepoDetails contains information about a repository
type RepoDetails struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

// Global variables for flags
var (
	configFile   string
	timeout      int
	downloadFlag string
	deployPath   string
	tagOnly      bool
	dateOnly     bool
)

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %v", err)
	}
	defer file.Close()

	config := &Config{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("error decoding config file: %v", err)
	}

	return config, nil
}

func saveConfig(config *Config, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating config file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("error encoding config file: %v", err)
	}

	return nil
}

func downloadAsset(baseURL, owner, repo, assetName, filePath string) error {
	// Fetch releases for the repository
	releases, err := gitearelease.GetReleases(gitearelease.ReleaseToFetch{
		BaseURL: baseURL,
		User:    owner,
		Repo:    repo,
		Latest:  true, // Get only the latest release
	})
	if err != nil {
		return fmt.Errorf("error getting releases: %v", err)
	}

	if len(releases) == 0 {
		return fmt.Errorf("no releases found for %s/%s", owner, repo)
	}

	// Get the latest release
	latestRelease := releases[0]

	// Find the asset by name
	var assetURL string
	var assetSize int64
	var found bool
	for _, asset := range latestRelease.Assets {
		if asset.Name == assetName {
			assetURL = asset.BrowserDownloadURL
			assetSize = asset.Size
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("asset %s not found in release %s", assetName, latestRelease.Name)
	}

	// Download the asset
	resp, err := http.Get(assetURL)
	if err != nil {
		return fmt.Errorf("error downloading asset: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error downloading asset, status: %s", resp.Status)
	}

	// Create the output file
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating output file: %v", err)
	}
	defer out.Close()

	// Create and start progress bar
	bar := pb.Full.Start64(assetSize)
	bar.Set(pb.Bytes, true)
	bar.SetTemplateString(`{{with string . "prefix"}}{{.}} {{end}}{{counters . }} {{bar . }} {{percent . }} {{speed . }} {{with string . "suffix"}}{{.}}{{end}}`)
	bar.Set("prefix", "Downloading:")
	bar.Set("suffix", fmt.Sprintf("[%s]", assetName))

	// Create proxy reader for progress bar
	barReader := bar.NewProxyReader(resp.Body)

	// Copy with progress bar
	_, err = io.Copy(out, barReader)
	bar.Finish()

	if err != nil {
		return fmt.Errorf("error writing to output file: %v", err)
	}

	return nil
}

func showAvailableRepos() error {
	config, err := loadConfig(configFile)
	if err == nil && len(config.Repos) > 0 {
		fmt.Println("Available repository aliases:")
		for alias := range config.Repos {
			fmt.Printf("  %s\n", alias)
		}
		return fmt.Errorf("please specify one of the repository aliases listed above")
	}
	return fmt.Errorf("repository alias is required")
}

func main() {
	// Set a reasonable default timeout
	gitearelease.SetHTTPTimeout(15 * time.Second)

	var rootCmd = &cobra.Command{
		Use:   "gitea-release",
		Short: "Interact with Gitea releases",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Set the HTTP timeout if specified
			if timeout > 0 {
				gitearelease.SetHTTPTimeout(time.Duration(timeout) * time.Second)
			}
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "gitea-release.json", "Path to the configuration file")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 15, "HTTP timeout in seconds for API requests")

	// Repo command
	var repoCmd = &cobra.Command{
		Use:   "repo",
		Short: "Manage repositories",
	}

	// Repo add command
	var urlFlag, ownerFlag, nameFlag, aliasFlag string
	var repoAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Add a repository to the configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If alias is not provided, use the repository name
			if aliasFlag == "" {
				aliasFlag = nameFlag
			}

			// Load existing config if available
			config, err := loadConfig(configFile)
			if err != nil {
				// If config doesn't exist, create a new one
				if os.IsNotExist(err) {
					config = &Config{
						GiteaURL: urlFlag, // use the provided URL initially
						Repos:    make(map[string]RepoDetails),
					}
				} else {
					return err
				}
			}

			// Check if url flag is an existing alias in the config
			var giteaURL string
			if _, exists := config.Repos[urlFlag]; exists {
				// Use the same Gitea URL as the referenced repo
				giteaURL = config.GiteaURL
				fmt.Printf("Using Gitea URL from existing alias '%s'\n", urlFlag)
			} else {
				// Use the URL provided in the flag
				giteaURL = urlFlag
			}

			// Try to check if the repository exists, but proceed even if we get an error
			// since we're working with public repos which may exist but have limited API access
			_, _ = gitearelease.GetRepositories(gitearelease.RepositoriesToFetch{
				BaseURL: giteaURL,
				User:    ownerFlag,
			})

			// Skip the existence check - we'll assume the repo exists
			// and let the user verify manually

			// Update config
			config.GiteaURL = giteaURL // Keep the URL consistent for all repos
			config.Repos[aliasFlag] = RepoDetails{
				Owner: ownerFlag,
				Name:  nameFlag,
			}

			// Save config
			if err := saveConfig(config, configFile); err != nil {
				return err
			}

			fmt.Printf("Repository %s/%s added with alias %s\n", ownerFlag, nameFlag, aliasFlag)
			return nil
		},
	}

	repoAddCmd.Flags().StringVar(&urlFlag, "url", "", "Gitea URL or an existing repository alias")
	repoAddCmd.Flags().StringVar(&ownerFlag, "owner", "", "Repository owner")
	repoAddCmd.Flags().StringVar(&nameFlag, "name", "", "Repository name")
	repoAddCmd.Flags().StringVar(&aliasFlag, "alias", "", "Repository alias (defaults to repository name if not provided)")
	repoAddCmd.MarkFlagRequired("url")
	repoAddCmd.MarkFlagRequired("owner")
	repoAddCmd.MarkFlagRequired("name")

	// Repo list command
	var repoListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all configured repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadConfig(configFile)
			if err != nil {
				return err
			}

			fmt.Println("Configured repositories:")
			for alias, repo := range config.Repos {
				fmt.Printf("  %s: %s/%s\n", alias, repo.Owner, repo.Name)
			}
			return nil
		},
	}

	// List releases command
	var listCmd = &cobra.Command{
		Use:   "list [repo-alias]",
		Short: "List all releases for a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return showAvailableRepos()
			}

			repoAlias := args[0]

			// Load config
			config, err := loadConfig(configFile)
			if err != nil {
				return err
			}

			repoDetails, ok := config.Repos[repoAlias]
			if !ok {
				return fmt.Errorf("repository alias %s not found", repoAlias)
			}

			// Get releases using the package
			releases, err := gitearelease.GetReleases(gitearelease.ReleaseToFetch{
				BaseURL: config.GiteaURL,
				User:    repoDetails.Owner,
				Repo:    repoDetails.Name,
				Latest:  false, // Get all releases
			})
			if err != nil {
				return fmt.Errorf("error getting releases: %v", err)
			}

			if len(releases) == 0 {
				fmt.Printf("No releases found for %s/%s\n", repoDetails.Owner, repoDetails.Name)
				return nil
			}

			fmt.Printf("Releases for %s/%s:\n", repoDetails.Owner, repoDetails.Name)
			for _, release := range releases {
				fmt.Printf("  %s (Published: %s)\n", release.Name, release.PublishedAt)
				fmt.Printf("    Tag: %s\n", release.TagName)
				fmt.Printf("    Assets:\n")
				for _, asset := range release.Assets {
					fmt.Printf("      %s (Size: %d bytes)\n", asset.Name, asset.Size)
				}
				fmt.Println()
			}

			return nil
		},
	}

	// Fetch release command (replacing the "latest" command)
	var fetchCmd = &cobra.Command{
		Use:   "fetch [repo-alias] [release-tag-or-latest]",
		Short: "Fetch a specific or the latest release for a repository",
		Long:  "Fetch a specific release by tag/title or the latest release for a repository",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return showAvailableRepos()
			}

			repoAlias := args[0]
			releaseIdentifier := "latest" // Default to latest release

			// If a second argument is provided, use it as the release tag/title
			if len(args) > 1 {
				releaseIdentifier = args[1]
			}

			// Load config
			config, err := loadConfig(configFile)
			if err != nil {
				return err
			}

			repoDetails, ok := config.Repos[repoAlias]
			if !ok {
				return fmt.Errorf("repository alias %s not found", repoAlias)
			}

			var releases []gitearelease.Release
			var targetRelease gitearelease.Release
			var found bool

			// Get releases using the package
			if releaseIdentifier == "latest" {
				releases, err = gitearelease.GetReleases(gitearelease.ReleaseToFetch{
					BaseURL: config.GiteaURL,
					User:    repoDetails.Owner,
					Repo:    repoDetails.Name,
					Latest:  true, // Get only the latest release
				})
				if err != nil {
					return fmt.Errorf("error getting releases: %v", err)
				}

				if len(releases) == 0 {
					return fmt.Errorf("no releases found for %s/%s", repoDetails.Owner, repoDetails.Name)
				}

				// Get latest release
				targetRelease = releases[0]
				found = true
			} else {
				// Get all releases to find the specified one
				releases, err = gitearelease.GetReleases(gitearelease.ReleaseToFetch{
					BaseURL: config.GiteaURL,
					User:    repoDetails.Owner,
					Repo:    repoDetails.Name,
					Latest:  false, // Get all releases
				})
				if err != nil {
					return fmt.Errorf("error getting releases: %v", err)
				}

				// Find the release by tag or title
				for _, release := range releases {
					if release.TagName == releaseIdentifier || release.Name == releaseIdentifier {
						targetRelease = release
						found = true
						break
					}
				}

				if !found {
					return fmt.Errorf("release with tag or title '%s' not found", releaseIdentifier)
				}
			}

			if downloadFlag != "" {
				// Check if the asset exists
				var assetExists bool
				for _, asset := range targetRelease.Assets {
					if asset.Name == downloadFlag {
						assetExists = true
						break
					}
				}

				if !assetExists {
					return fmt.Errorf("asset %s not found in release %s", downloadFlag, targetRelease.Name)
				}

				// Default download path is current directory with asset name
				downloadPath := downloadFlag

				// If deploy path is specified, use it
				if deployPath != "" {
					// Create deploy directory if it doesn't exist
					if err := os.MkdirAll(deployPath, 0755); err != nil {
						return fmt.Errorf("error creating deploy directory: %v", err)
					}

					// First download to a temporary location
					tempPath := filepath.Join(os.TempDir(), downloadFlag)
					if err := downloadAsset(config.GiteaURL, repoDetails.Owner, repoDetails.Name, downloadFlag, tempPath); err != nil {
						return err
					}

					// Then move to deploy location
					finalPath := filepath.Join(deployPath, downloadFlag)
					if err := os.Rename(tempPath, finalPath); err != nil {
						return fmt.Errorf("error deploying file: %v", err)
					}

					fmt.Printf("\nAsset %s from release %s has been downloaded and deployed to %s\n",
						downloadFlag, targetRelease.Name, finalPath)
				} else {
					// Just download to current directory
					if err := downloadAsset(config.GiteaURL, repoDetails.Owner, repoDetails.Name, downloadFlag, downloadPath); err != nil {
						return err
					}

					absPath, _ := filepath.Abs(downloadPath)
					fmt.Printf("\nAsset %s from release %s has been downloaded to %s\n",
						downloadFlag, targetRelease.Name, absPath)
				}

				return nil
			}

			// Handle simplified output formats
			if tagOnly {
				// Just print the tag with no additional text
				fmt.Print(targetRelease.TagName)
				return nil
			}

			if dateOnly {
				// Just print the date with no additional text
				fmt.Print(targetRelease.PublishedAt)
				return nil
			}

			// Display release info
			fmt.Printf("Release for %s/%s:\n", repoDetails.Owner, repoDetails.Name)
			fmt.Printf("  Name: %s\n", targetRelease.Name)
			fmt.Printf("  Tag: %s\n", targetRelease.TagName)
			fmt.Printf("  Published: %s\n", targetRelease.PublishedAt)
			fmt.Printf("  Assets:\n")
			for _, asset := range targetRelease.Assets {
				fmt.Printf("    %s (Size: %d bytes)\n", asset.Name, asset.Size)
			}

			return nil
		},
	}

	fetchCmd.Flags().StringVar(&downloadFlag, "download", "", "Download a specific asset from the release")
	fetchCmd.Flags().StringVar(&deployPath, "deploy", "", "Path to deploy the downloaded asset")
	fetchCmd.Flags().BoolVar(&tagOnly, "tag", false, "Output only the tag name with no additional text")
	fetchCmd.Flags().BoolVar(&dateOnly, "date", false, "Output only the published date with no additional text")

	// Add commands to their parents
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoListCmd)
	rootCmd.AddCommand(repoCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(fetchCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
