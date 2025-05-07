# Slack Integration Package

This package provides functionality for sending messages to Slack channels using the BuildMyChat.ai Hub-and-Spoke model.

## Usage

### Sending Messages

There are three ways to send messages to Slack:

1. **Using Interface Configuration**:
   ```go
   // When you have the interface configuration JSON
   err := slack.SendMessageUsingInterfaceConfig(ctx, interfaceConfig, channelID, message, threadTs)
   ```

2. **Using Interface ID**:
   ```go
   // When you have the interface ID and organization ID
   err := slack.SendMessageUsingInterfaceID(ctx, storeInstance, interfaceID, orgID, channelID, message, threadTs)
   ```

3. **Using Bot Token Directly**:
   ```go
   // When you already have the bot token
   err := slack.SendMessageToChannel(ctx, botToken, channelID, message, threadTs)
   ```

### Configuration Structure

The Slack interface configuration has the following structure:

```json
{
  "bot_token": "xoxb-your-slack-bot-token",
  "signing_secret": "your-slack-signing-secret"
}
```

### Helper Functions

- `ExtractTokenFromConfig`: Extracts the bot token from a configuration JSON
  ```go
  token, err := slack.ExtractTokenFromConfig(configJSON)
  ```

## Integration with Chat Service

The `sendMessageToSlack` function in the Chat Service uses `SendMessageUsingInterfaceConfig` to send messages to Slack channels. This approach directly uses the interface configuration stored in the database, which includes the bot token.

## Security Considerations

- The bot token is stored in the interface configuration in the database
- The interface configuration is stored as JSONB in the `interfaces` table
- Make sure to properly secure access to the interface configuration 