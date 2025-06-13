# zammad-go-mcp

MCP Server for accessing the Zammad API. 

This server enables:

- Reading ticket and user lists.
- Fetching details for specific tickets and users.
- Searching for tickets and users.
- Creating new tickets.
- Adding notes (articles) to existing tickets.
- Retrieving communication history (articles) for tickets.

## Capabilities

The server exposes the following MCP Resources and Tools:

Resources allow the AI to read data from Zammad in a structured way using URIs.

*   **`zammad://tickets`**
    *   **Name:** List Tickets
    *   **Description:** Lists all tickets accessible by the configured API token.
    *   **MIME Type:** `application/json`
*   **`zammad://tickets/{ticket_id}`** (Template)
    *   **Name:** Show Ticket (Resource)
    *   **Description:** Shows details for a specific ticket identified by its `{ticket_id}`.
    *   **MIME Type:** `application/json`
*   **`zammad://users`**
    *   **Name:** List Users
    *   **Description:** Lists all users accessible by the configured API token.
    *   **MIME Type:** `application/json`
*   **`zammad://users/{user_id}`** (Template)
    *   **Name:** Show User (Resource)
    *   **Description:** Shows details for a specific user identified by their `{user_id}`.
    *   **MIME Type:** `application/json`

### Tools

Tools allow the AI to perform actions or specific queries within Zammad.

*   **`create_ticket`**: Creates a new ticket in Zammad.
    *   Requires: `title`, `group`, `customer` (email or user ID), `body`.
    *   Optional: `type` (article type, default: "note"), `internal` (boolean, default: false).
*   **`search_tickets`**: Searches for tickets based on a query string.
    *   Requires: `query`.
    *   Optional: `limit` (default: 50).
*   **`add_note_to_ticket`**: Adds an internal note (article) to an existing ticket.
    *   Requires: `ticket_id`, `body`.
    *   Optional: `internal` (boolean, default: true).
*   **`reply_to_ticket`**: Reply to a ticket by email.
    *   Requires: `ticket_id`, `body`.
    *   Optional: `to` (recipient email, defaults to ticket customer), `cc`, `subject` (defaults to "Re: [ticket title]"), `internal` (boolean, default: false).
*   **`get_ticket`**: Retrieves details for a specific ticket by its ID.
    *   Requires: `ticket_id`.
*   **`get_user`**: Retrieves details for a specific user by their ID.
    *   Requires: `user_id`.
    *   Optional: `with_extended_data` (boolean, default: false). If true, returns all user fields including custom fields. If false, returns only standard fields.
*   **`search_users`**: Searches for users based on a query string (e.g., email, login, name).
    *   Requires: `query`.
    *   Optional: `limit` (default: 50).
*   **`get_ticket_articles`**: Retrieves all articles (communications) for a specific ticket.
    *   Requires: `ticket_id`.
*   **`close_ticket`**: Close a ticket by setting its state to 'closed'.
    *   Requires: `ticket_id`.
    *   Optional: `note` (a closing note to add to the ticket as an internal note).
*   **`assign_ticket`**: Assign a ticket to a specific agent user.
    *   Requires: `ticket_id`, `agent_id`.
    *   Optional: `note` (an assignment note to add to the ticket as an internal note).
*   **`add_tag_to_ticket`**: Add a tag to a ticket.
    *   Requires: `ticket_id`, `tag_name`.
*   **`get_ticket_tags`**: Get all tags currently assigned to a specific ticket.
    *   Requires: `ticket_id`.
*   **`list_all_tags`**: List all tags available in the Zammad system.
    *   Requires: `admin.tag` permission.
*   **`search_tags`**: Search for tags by name in the Zammad system.
    *   Requires: `search_term`.

### Text Module Tools

*   **`list_text_modules`**: List all text modules available in the Zammad system.
    *   No parameters required.
*   **`get_text_module`**: Get details of a specific text module by ID.
    *   Requires: `text_module_id`.
*   **`search_text_modules`**: Search for text modules by name, keywords, or content.
    *   Requires: `search_term`.

## Prerequisites

*   **Go:** Version 1.24 or higher installed.
*   **Zammad Instance:** Access to a running Zammad instance (URL).
*   **Zammad API Token:** An API token generated within your Zammad instance with sufficient permissions.

## Getting a Zammad API Token

You need to generate an API token within Zammad to allow this MCP server to authenticate and interact with the API.

1.  **Log in** to your Zammad instance with an administrator account (or an account that has permission to manage API tokens).
2.  Navigate to your **Profile** settings (usually by clicking your avatar/initials in the bottom-left).
3.  Go to the **Token Access** section.
4.  Click **"Create"** or the relevant button to generate a new token.
5.  Give the token a descriptive **Label** (e.g., "Claude MCP Server").
6.  **Crucially, assign the necessary permissions.** Based on the tools provided, you will likely need permissions like:
    *   `ticket.agent` (or `ticket.customer` depending on use case) - To view, create, search tickets and add articles.
    *   `user.reader` - To view and search users.
    *   *(Optional)* `admin.user` might be needed for broader user searches or modifications if you add those tools later. Review Zammad's permission documentation for specifics.
7.  Click **"Create"** or **"Save"**.
8.  **Immediately copy the generated token.** Zammad will only show you the token *once*. Store it securely.

## Installation & Setup

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/arush15june/zammad-mcp-go.git
    cd zammad-mcp-go
    ```

2.  **Build the binary:**
    ```bash
    go build -o zammad-mcp-go main.go
    ```
    
    This will create an executable file named `zammad-mcp-go` (or `zammad-mcp-go.exe` on Windows) in the current directory.


# Claude Desktop Configuration

```json
{
  "mcpServers": {
        "zammad": {
            "command": "<path-to>/zammad-go-mcp.exe",
            "args": [],
            "env": {
                "ZAMMAD_URL": "<zammad_url>",
                "ZAMMAD_TOKEN": "<zammad_token>"
            }
        }
    }
}
```