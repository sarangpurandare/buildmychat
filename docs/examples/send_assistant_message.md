# Sending Assistant Messages with Interface Integration

This document explains how to use the assistant message API with the optional interface integration.

## API Endpoint

```
POST /v1/chats/{chatID}/messages/assistant
```

## Request Body

```json
{
  "message": "Hello, I'm the assistant!",
  "metadata": {
    "source": "example-source",
    "confidence": 0.95
  },
  "send_to_interface": true
}
```

### Parameters

- `message` (required): The content of the assistant's message
- `metadata` (optional): Additional data about the message (JSON object)
- `send_to_interface` (optional): Boolean flag to send the message to the associated interface

## How It Works

1. When `send_to_interface` is set to `true`, the API will:
   - Add the assistant message to the chat history
   - If the chat has an associated interface (e.g., Slack), send the message to that interface
   - Return the updated chat

2. When `send_to_interface` is omitted or set to `false`, the API will:
   - Add the assistant message to the chat history
   - Return the updated chat without sending to any interface

## Example Usage

### Curl Example

```bash
curl -X POST \
  http://localhost:8080/v1/chats/3f4ecfff-7923-43c9-838e-6d2e7d59ecea/messages/assistant \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer YOUR_JWT_TOKEN' \
  -d '{
    "message": "Based on your question, I found this information...",
    "metadata": {
      "sources": ["knowledge-base-1", "document-123"],
      "confidence": 0.87
    },
    "send_to_interface": true
  }'
```

### JavaScript Example

```javascript
const response = await fetch(
  'http://localhost:8080/v1/chats/3f4ecfff-7923-43c9-838e-6d2e7d59ecea/messages/assistant',
  {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer YOUR_JWT_TOKEN'
    },
    body: JSON.stringify({
      message: 'Based on your question, I found this information...',
      metadata: {
        sources: ['knowledge-base-1', 'document-123'],
        confidence: 0.87
      },
      send_to_interface: true
    })
  }
);

const data = await response.json();
console.log(data);
```

### Go Example

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func main() {
	type Metadata struct {
		Sources    []string `json:"sources"`
		Confidence float64  `json:"confidence"`
	}

	type AssistantMessageRequest struct {
		Message         string    `json:"message"`
		Metadata        *Metadata `json:"metadata,omitempty"`
		SendToInterface *bool     `json:"send_to_interface,omitempty"`
	}

	// Create the request
	sendToInterface := true
	req := AssistantMessageRequest{
		Message: "Based on your question, I found this information...",
		Metadata: &Metadata{
			Sources:    []string{"knowledge-base-1", "document-123"},
			Confidence: 0.87,
		},
		SendToInterface: &sendToInterface,
	}

	// Convert to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Create the HTTP request
	chatID := "3f4ecfff-7923-43c9-838e-6d2e7d59ecea"
	request, err := http.NewRequest(
		"POST",
		fmt.Sprintf("http://localhost:8080/v1/chats/%s/messages/assistant", chatID),
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Set headers
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer YOUR_JWT_TOKEN")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Process the response
	fmt.Printf("Response Status: %s\n", resp.Status)
}
``` 