package intervalled_broadcasts

// This file would normally contain the implementation of any event handlers or
// specific functionality, but since this extension is solely focused on sending
// periodic broadcasts, all the required functionality is already implemented
// in the generic.go file through the Initialize, startBroadcasts, and Shutdown methods.

// The extension works by:
// 1. Setting up a ticker at the configured interval
// 2. Cycling through the configured messages in order
// 3. Sending each message to the server via RCON's AdminBroadcast command
// 4. Adding the configured prefix to each message if specified
// 5. Properly shutting down the ticker when the extension is stopped

// Any additional functionality can be added here as needed.
