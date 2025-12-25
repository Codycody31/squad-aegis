# Example Custom Plugin

This is an example custom plugin for Squad Aegis demonstrating how to build plugins using the Plugin SDK.

## Features Demonstrated

- Plugin manifest declaration
- Feature-based API access
- Event handling
- Logging
- Goroutine tracking
- Graceful shutdown

## Building the Plugin

### Prerequisites

- Go 1.21 or later
- Squad Aegis source code (for SDK imports)

### Build Steps

1. **Build the plugin:**
   ```bash
   make build
   ```
   
   This creates `example_custom_plugin.so`

2. **Generate signing keys (optional, for development):**
   ```bash
   make generate-keys
   ```
   
   This creates `private.key` and `public.key`

3. **Sign the plugin (optional):**
   ```bash
   make sign
   ```
   
   This creates `example_custom_plugin.so.sig`

4. **Build and sign in one step:**
   ```bash
   make release
   ```

## Plugin Structure

```go
// PluginExport is the required exported symbol
var PluginExport plugin_sdk.PluginSDK = &ExamplePlugin{}

type ExamplePlugin struct {
    baseAPI plugin_sdk.BaseAPI
    // ... your plugin fields
}

// Required methods:
func (p *ExamplePlugin) GetManifest() plugin_sdk.PluginManifest { ... }
func (p *ExamplePlugin) Initialize(baseAPI plugin_sdk.BaseAPI) error { ... }
func (p *ExamplePlugin) Shutdown() error { ... }
```

## Manifest Declaration

The manifest declares what your plugin needs and provides:

```go
func (p *ExamplePlugin) GetManifest() plugin_sdk.PluginManifest {
    return plugin_sdk.PluginManifest{
        ID:          "example_custom_plugin",
        Name:        "Example Custom Plugin",
        Version:     "1.0.0",
        Author:      "Your Name",
        SDKVersion:  plugin_sdk.APIVersion,
        Description: "What your plugin does",
        
        // Declare features you need
        RequiredFeatures: []plugin_sdk.FeatureID{
            plugin_sdk.FeatureLogging,
            plugin_sdk.FeatureEventHandling,
        },
        
        // Declare permissions you need
        RequiredPermissions: []plugin_sdk.PermissionID{
            plugin_sdk.PermissionEventPublish,
        },
    }
}
```

## Accessing APIs

Use the feature gateway to access APIs:

```go
func (p *ExamplePlugin) Initialize(baseAPI plugin_sdk.BaseAPI) error {
    // Get an API by feature ID
    logAPI, err := baseAPI.GetFeatureAPI(plugin_sdk.FeatureLogging)
    if err != nil {
        return err
    }
    
    log, ok := logAPI.(plugin_sdk.LogAPI)
    if !ok {
        return fmt.Errorf("logging API has incorrect type")
    }
    
    log.Info("Plugin initialized", nil)
    return nil
}
```

## Available Features

- `FeatureLogging` - Logging capabilities
- `FeatureEventHandling` - Subscribe to and publish events
- `FeatureRCON` - Send RCON commands
- `FeatureDatabaseAccess` - Read/write plugin data
- `FeatureServerAPI` - Get server info, players, squads
- `FeatureAdminAPI` - Manage admins
- `FeatureConnectors` - Access Discord, etc.
- `FeatureCommands` - Expose user-executable commands

## Resource Management

### Goroutines

Always spawn goroutines through the base API for tracking:

```go
p.baseAPI.SpawnGoroutine(func() {
    // Your goroutine code
})
```

### Context

Use the plugin context for graceful shutdown:

```go
ctx := p.baseAPI.GetContext()

// In your goroutine
for {
    select {
    case <-ctx.Done():
        return // Plugin is shutting down
    case <-ticker.C:
        // Do work
    }
}
```

## Uploading the Plugin

1. Build and sign your plugin
2. Navigate to the Squad Aegis web interface
3. Go to Plugins → Custom Plugins
4. Click "Upload Plugin"
5. Select your `.so` file and provide the signature
6. Wait for admin approval
7. Grant required permissions
8. Enable and configure the plugin

## Security

### Signing Plugins

Production plugins should be signed with a trusted key:

```bash
# Generate a key pair (once)
go run generate_keys.go

# Sign your plugin
go run sign_plugin.go example_custom_plugin.so private.key

# Upload public.key to Squad Aegis via the web interface
# Then upload your plugin with the signature
```

### Permissions

Plugins must declare and be granted permissions:

- `rcon.access` - Send RCON commands
- `database.read` - Read from database
- `database.write` - Write to database
- `player.management` - Kick/ban players
- `admin.management` - Manage admins
- `event.publish` - Publish events
- `connector.access` - Use connectors

### Sandbox Limits

Plugins run with resource limits:

- **Memory**: Default 512 MB
- **Goroutines**: Default 100
- **CPU Time**: Configurable

Exceeding limits will cause the plugin to be stopped.

## Best Practices

1. **Error Handling**: Always check errors from API calls
2. **Logging**: Use the LogAPI instead of fmt.Println
3. **Graceful Shutdown**: Watch the context and clean up resources
4. **Resource Limits**: Be mindful of memory and goroutine usage
5. **Testing**: Test your plugin thoroughly before deployment
6. **Versioning**: Use semantic versioning
7. **Documentation**: Document your plugin's configuration and features

## Troubleshooting

### Plugin won't load

- Check that `PluginExport` is defined and exported
- Verify SDK version compatibility
- Check server logs for detailed errors

### Permission denied

- Ensure all required permissions are granted
- Check that features are declared in manifest

### Plugin crashes

- Check for panics in your code
- Verify API types after casting
- Monitor sandbox resource usage

## Example Plugins

See the `examples/test_plugins/` directory for more examples:

- `simple_logger/` - Minimal logging plugin
- `event_processor/` - Event handling example
- `command_plugin/` - Command execution example

## Support

- Documentation: https://squad-aegis.docs.com
- Issues: https://github.com/codycody31/squad-aegis/issues
- Discord: https://discord.gg/squad-aegis

