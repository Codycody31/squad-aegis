# Discord Connector Setup Guide

## Prerequisites

1. **Super Admin Access**: You must be logged in as a super admin to manage global connectors
2. **Discord Bot**: You need to create a Discord bot and get its token
3. **Guild ID**: You need the Discord server (guild) ID where the bot will operate

## Step 1: Create a Discord Bot

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application" and give it a name (e.g., "Squad Aegis Bot")
3. Go to the "Bot" section in the sidebar
4. Click "Add Bot" 
5. Copy the **Bot Token** (you'll need this for the connector configuration)
6. Under "Privileged Gateway Intents", enable:
   - **Server Members Intent** (for member management)
   - **Message Content Intent** (for reading chat messages)

## Step 2: Add Bot to Your Discord Server

1. In the Discord Developer Portal, go to "OAuth2" → "URL Generator"
2. Select scopes:
   - `bot`
   - `applications.commands`
3. Select bot permissions:
   - `Send Messages`
   - `Use Slash Commands`
   - `Embed Links`
   - `Mention Everyone` (if you want @here/@everyone pings)
   - `Manage Roles` (if you want role management features)
4. Copy the generated URL and open it in your browser
5. Select your Discord server and authorize the bot

## Step 3: Get Your Guild ID

1. In Discord, enable Developer Mode:
   - User Settings → Advanced → Developer Mode (toggle on)
2. Right-click on your Discord server name
3. Click "Copy Server ID" - this is your **Guild ID**

## Step 4: Configure the Connector in Squad Aegis

1. **Login as Super Admin**: Make sure you're logged in with super admin privileges
2. **Navigate to Connectors**: Go to the "Connectors" page in the main navigation
3. **Add Discord Connector**:
   - Click "Add Connector"
   - Select "Discord" from the dropdown
   - Fill in the configuration:
     - **Token**: Your Discord bot token (starts with something like `MTA...`)
     - **Guild ID**: Your Discord server ID (numbers only)
4. **Create the Connector**: Click "Create Connector"

## Step 5: Verify the Connection

After creating the connector, you should see:
- Status: "running" (green badge)
- No errors in the "Last Error" column

If you see an error status:
- Check that your bot token is correct
- Verify the guild ID is correct
- Ensure the bot has been added to your Discord server
- Check that the bot has the necessary permissions

## Step 6: Configure Server Plugins

Once the Discord connector is running:
1. Go to any server's "Plugins" page
2. Click "Add Plugin"
3. Select "Discord Admin Requests"
4. Configure the plugin with:
   - **Channel ID**: The Discord channel where admin requests should be sent
   - **Ping Groups**: Role IDs to ping (optional)
   - **Other settings**: Customize as needed

## Getting Channel and Role IDs

**Channel ID:**
1. Right-click on the Discord channel where you want admin requests
2. Click "Copy Channel ID"

**Role ID:**
1. In Discord server settings, go to "Roles"
2. Right-click on the role you want to ping
3. Click "Copy Role ID"

## Troubleshooting

**"Plugin manager not available" error:**
- Ensure the server is running with the plugin system enabled

**Discord connector not in the list:**
- Check server logs for connector registration errors
- Verify you're logged in as super admin
- Make sure you're on the `/connectors` page, not server-specific plugins

**Bot not responding:**
- Check connector status in the UI
- Verify bot permissions in Discord
- Check server logs for Discord API errors

**Plugin creation fails:**
- Ensure Discord connector is running (green status)
- Verify the connector has no errors
- Check that required connector dependencies are met
