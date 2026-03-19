package tools

// Registry contains all available tool definitions
var Registry = map[string]*Tool{
	// Bootstrap - composer must be installed first (from getcomposer.org)
	"composer": {
		Name:        "composer",
		Description: "Dependency Manager for PHP",
		Type:        ToolTypeBootstrap,
		VersionURL:  "https://getcomposer.org/versions",
	},

	// Binaries from GitHub releases
	"symfony": {
		Name:        "symfony",
		Description: "Symfony CLI - tool to build PHP applications",
		Type:        ToolTypeBinary,
		GitHubRepo:  "symfony-cli/symfony-cli",
		PlatformAssets: map[string]string{
			"darwin-arm64": "symfony-cli_darwin_arm64.tar.gz",
			"darwin-amd64": "symfony-cli_darwin_amd64.tar.gz",
		},
	},
	"castor": {
		Name:        "castor",
		Description: "DX-oriented task runner built in PHP",
		Type:        ToolTypeBinary,
		GitHubRepo:  "jolicode/castor",
		PlatformAssets: map[string]string{
			"darwin-arm64": "castor.darwin-arm64",
			"darwin-amd64": "castor.darwin-amd64",
		},
	},

	// Phars installed via composer
	"phpstan": {
		Name:         "phpstan",
		Description:  "PHP Static Analysis Tool",
		Type:         ToolTypePhar,
		ComposerPkg:  "phpstan/phpstan",
		PharInVendor: "phpstan/phpstan/phpstan.phar",
	},
	"php-cs-fixer": {
		Name:         "php-cs-fixer",
		Description:  "PHP Coding Standards Fixer",
		Type:         ToolTypePhar,
		ComposerPkg:  "friendsofphp/php-cs-fixer",
		PharInVendor: "friendsofphp/php-cs-fixer/php-cs-fixer.phar",
	},
	"psalm": {
		Name:         "psalm",
		Description:  "PHP Static Analysis Tool by Vimeo",
		Type:         ToolTypePhar,
		ComposerPkg:  "vimeo/psalm",
		PharInVendor: "vimeo/psalm/psalm.phar",
	},
	"laravel": {
		Name:         "laravel",
		Description:  "Laravel Installer",
		Type:         ToolTypePhar,
		ComposerPkg:  "laravel/installer",
		PharInVendor: "laravel/installer/bin/laravel", // Not a phar, but a PHP script
	},
	"deployer": {
		Name:         "deployer",
		Description:  "Deployment tool for PHP",
		Type:         ToolTypePhar,
		ComposerPkg:  "deployer/deployer",
		PharInVendor: "deployer/deployer/bin/dep", // Not a phar, but a PHP script
	},
}

// GetTool returns a tool definition by name
func GetTool(name string) *Tool {
	return Registry[name]
}

// GetAllTools returns all tool definitions
func GetAllTools() map[string]*Tool {
	return Registry
}

// IsKnownTool checks if a name is a known tool
func IsKnownTool(name string) bool {
	_, exists := Registry[name]
	return exists
}
