Gitea Release CLI
A command-line interface for interacting with Gitea releases, allowing you to list repositories, fetch releases, and download assets from public Gitea repositories.
Features

Manage repositories from multiple Gitea instances
List all releases for a repository
Fetch the latest or a specific release by tag/title
Download release assets with progress bars and ETA
Deploy downloaded assets to specific locations
Get clean tag/date output for scripting purposes

Installation
Prerequisites

Go 1.16 or higher

Installing from source
bash# Clone this repository
git clone https://github.com/yourusername/gitea-release-cli.git
cd gitea-release-cli

# Install dependencies
go get github.com/earentir/gitearelease
go get github.com/spf13/cobra
go get github.com/cheggaaa/pb/v3

# Build the binary
go build -o gitea-release

# Optionally, install to your PATH
go install
Configuration
The CLI uses a configuration file (default: gitea-release.json) to store information about Gitea servers and repositories. You can specify a different config file with the --config flag.
Example configuration file:
json{
  "gitea_url": "https://gitea.example.com",
  "repos": {
    "myrepo": {
      "owner": "username",
      "name": "repository"
    }
  }
}
Usage
Managing Repositories
Add a repository to your configuration:
bash# Adding a repository using a direct Gitea URL
gitea-release repo add --url "https://gitea.example.com" --owner "username" --name "repository" --alias "myrepo"

# Adding a repository using an existing repository's Gitea URL
gitea-release repo add --url "myrepo" --owner "username" --name "another-repo" --alias "another"
List all configured repositories:
bashgitea-release repo list
Listing Releases
List all releases for a repository:
bashgitea-release list myrepo
Fetching Releases
Fetch the latest release:
bashgitea-release fetch myrepo
# or explicitly
gitea-release fetch myrepo latest
Fetch a specific release by tag or title:
bashgitea-release fetch myrepo v1.0.0
Get only the tag of a release (useful for scripting):
bashgitea-release fetch myrepo --tag
Get only the published date:
bashgitea-release fetch myrepo --date
Downloading Assets
Download an asset from the latest release:
bashgitea-release fetch myrepo --download asset-name
Download an asset from a specific release:
bashgitea-release fetch myrepo v1.0.0 --download asset-name
Download and deploy an asset to a specific location:
bashgitea-release fetch myrepo --download asset-name --deploy /path/to/directory
Examples
Adding and listing repositories
bash# Add a repository
gitea-release repo add --url "https://gitea.example.com" --owner "username" --name "project" --alias "proj1"

# Add another repository from the same Gitea instance
gitea-release repo add --url "proj1" --owner "username" --name "another-project" --alias "proj2"

# List configured repositories
gitea-release repo list
Fetching release information and downloading assets
bash# List all releases for a repository
gitea-release list proj1

# Get the latest release info
gitea-release fetch proj1

# Get information for a specific release
gitea-release fetch proj1 v1.2.3

# Download an asset from the latest release
gitea-release fetch proj1 --download my-binary

# Download and deploy an asset
gitea-release fetch proj1 v1.2.3 --download my-binary --deploy /usr/local/bin
Using in scripts
bash# Get the latest tag for a repository
TAG=$(gitea-release fetch myrepo --tag)
echo "Latest version: $TAG"

# Get the published date of a specific release
DATE=$(gitea-release fetch myrepo v1.0.0 --date)
echo "Published on: $DATE"

# Download the latest version of a binary
gitea-release fetch myrepo --download app-binary --deploy /usr/local/bin
Global Flags

--config - Path to the configuration file (default: gitea-release.json)
--timeout - HTTP timeout in seconds for API requests (default: 15)

License
This project is licensed under the MIT License.
Acknowledgments

gitearelease - Go package for interacting with Gitea releases
Cobra - A Commander for modern Go CLI interactions
pb - Console progress bar for Go
