package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/AlessandroSechi/zammad-go" // Import the Zammad client
	"github.com/mark3labs/mcp-go/mcp"      // Import the MCP types
	"github.com/mark3labs/mcp-go/server"   // Import the MCP server
)

var (
	ErrResourceNotFound error = errors.New("resource not found")
)

// TextModule represents a Zammad text module
type TextModule struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Keywords  string `json:"keywords"`
	Content   string `json:"content"`
	Note      string `json:"note"`
	Active    bool   `json:"active"`
	GroupIDs  []int  `json:"group_ids"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	CreatedBy int    `json:"created_by_id"`
	UpdatedBy int    `json:"updated_by_id"`
}

var zammadClient *zammad.Client

func main() {
	// --- Zammad Client Setup ---
	zammadURL := os.Getenv("ZAMMAD_URL")
	zammadToken := os.Getenv("ZAMMAD_TOKEN")

	if zammadURL == "" || zammadToken == "" {
		log.Fatal("Error: ZAMMAD_URL and ZAMMAD_TOKEN environment variables must be set.")
	}

	zammadClient = zammad.New(zammadURL)
	zammadClient.Token = zammadToken

	// Verify connection (optional but recommended)
	_, err := zammadClient.UserMe()
	if err != nil {
		log.Fatalf("Failed to connect to Zammad API: %v", err)
	}
	log.Println("Successfully connected to Zammad API.")

	// --- MCP Server Setup ---
	mcpServer := server.NewMCPServer(
		"Zammad MCP Server", // Server Name
		"1.0.0",             // Server Version
		// Enable necessary capabilities
		server.WithResourceCapabilities(true, true), // Read resources, support list changes
		server.WithToolCapabilities(true),           // Expose tools, support list changes
		server.WithLogging(),                        // Enable MCP logging notifications
		server.WithRecovery(),                       // Recover from panics in handlers
		// Updated instructions to include user tools
		server.WithInstructions("This server provides access to Zammad tickets, users, and tags via resources and tools (e.g., create_ticket, get_ticket, search_tickets, get_user, search_users, add_tag_to_ticket, get_ticket_tags)."),
	)

	// --- Register MCP Resources ---
	registerResources(mcpServer)

	// --- Register MCP Tools ---
	registerTools(mcpServer) // This function now includes user tools

	// --- Start MCP Server ---
	log.Println("Starting Zammad MCP server via stdio...")
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// =====================================
// MCP Resource Registration & Handlers
// =====================================

func registerResources(s *server.MCPServer) {
	// 1. List Tickets Resource
	listTicketsResource := mcp.NewResource(
		"zammad://tickets", // URI for listing all tickets
		"List Tickets",
		mcp.WithResourceDescription("Lists all tickets accessible by the API token."),
		mcp.WithMIMEType("application/json"),
	)
	s.AddResource(listTicketsResource, handleListTickets)

	// 2. Show Ticket Resource (Dynamic via Template)
	showTicketTemplate := mcp.NewResourceTemplate(
		"zammad://tickets/{ticket_id}", // URI template
		"Show Ticket (Resource)",       // Renamed slightly to distinguish from tool
		mcp.WithTemplateDescription("Shows details for a specific ticket by its ID (via resource read)."),
		mcp.WithTemplateMIMEType("application/json"),
	)
	s.AddResourceTemplate(showTicketTemplate, handleShowTicket)

	// 3. List Users Resource
	listUsersResource := mcp.NewResource(
		"zammad://users",
		"List Users",
		mcp.WithResourceDescription("Lists all users accessible by the API token."),
		mcp.WithMIMEType("application/json"),
	)
	s.AddResource(listUsersResource, handleListUsers)

	// 4. Show User Resource (Dynamic via Template) <-- NEW RESOURCE
	showUserTemplate := mcp.NewResourceTemplate(
		"zammad://users/{user_id}", // URI template
		"Show User (Resource)",
		mcp.WithTemplateDescription("Shows details for a specific user by their ID (via resource read)."),
		mcp.WithTemplateMIMEType("application/json"),
	)
	s.AddResourceTemplate(showUserTemplate, handleShowUser) // Register new handler
}

// handleListTickets retrieves all tickets from Zammad.
func handleListTickets(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	log.Printf("Handling request for resource: %s", request.Params.URI)
	tickets, err := zammadClient.TicketList() // Consider pagination for large instances
	if err != nil {
		log.Printf("Error fetching tickets from Zammad: %v", err)
		return nil, fmt.Errorf("failed to fetch tickets: %w", err)
	}

	jsonData, err := json.MarshalIndent(tickets, "", "  ")
	if err != nil {
		log.Printf("Error marshalling tickets to JSON: %v", err)
		return nil, fmt.Errorf("failed to marshal tickets: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

// handleShowTicket retrieves details for a specific ticket via resource read.
func handleShowTicket(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	log.Printf("Handling request for resource: %s", request.Params.URI)

	ticketIDStr, ok := request.Params.Arguments["ticket_id"].(string)
	if !ok {
		log.Printf("Error: ticket_id not found or not a string in arguments: %v", request.Params.Arguments)
		return nil, fmt.Errorf("%w: invalid or missing ticket_id in URI", ErrResourceNotFound)
	}
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		log.Printf("Error converting ticket_id '%s' to int: %v", ticketIDStr, err)
		return nil, fmt.Errorf("%w: invalid ticket_id format: %w", ErrResourceNotFound, err)
	}

	ticket, err := zammadClient.TicketShow(ticketID)
	if err != nil {
		log.Printf("Error fetching ticket %d from Zammad: %v", ticketID, err)
		return nil, fmt.Errorf("%w: failed to fetch ticket %d: %w", ErrResourceNotFound, ticketID, err)
	}
	jsonData, err := json.MarshalIndent(ticket, "", "  ")
	if err != nil {
		log.Printf("Error marshalling ticket %d to JSON: %v", ticketID, err)
		return nil, fmt.Errorf("failed to marshal ticket %d: %w", ticketID, err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

// handleListUsers retrieves all users from Zammad.
func handleListUsers(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	log.Printf("Handling request for resource: %s", request.Params.URI)
	users, err := zammadClient.UserList() // Consider pagination
	if err != nil {
		log.Printf("Error fetching users from Zammad: %v", err)
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}
	jsonData, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		log.Printf("Error marshalling users to JSON: %v", err)
		return nil, fmt.Errorf("failed to marshal users: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

// handleShowUser retrieves details for a specific user via resource read. <-- NEW HANDLER
func handleShowUser(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	log.Printf("Handling request for resource: %s", request.Params.URI)

	userIDStr, ok := request.Params.Arguments["user_id"].(string)
	if !ok {
		log.Printf("Error: user_id not found or not a string in arguments: %v", request.Params.Arguments)
		return nil, fmt.Errorf("%w: invalid or missing user_id in URI", ErrResourceNotFound)
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		log.Printf("Error converting user_id '%s' to int: %v", userIDStr, err)
		return nil, fmt.Errorf("%w: invalid user_id format: %w", ErrResourceNotFound, err)
	}

	user, err := zammadClient.UserShow(userID)
	if err != nil {
		log.Printf("Error fetching user %d from Zammad: %v", userID, err)
		return nil, fmt.Errorf("%w: failed to fetch user %d: %w", ErrResourceNotFound, userID, err)
	}
	jsonData, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		log.Printf("Error marshalling user %d to JSON: %v", userID, err)
		return nil, fmt.Errorf("failed to marshal user %d: %w", userID, err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

// ==================================
// MCP Tool Registration & Handlers
// ==================================

func registerTools(s *server.MCPServer) {
	// --- Ticket Tools ---
	createTicketTool := mcp.NewTool("create_ticket" /* ... */)
	s.AddTool(createTicketTool, handleCreateTicket)

	searchTicketsTool := mcp.NewTool("search_tickets" /* ... */)
	s.AddTool(searchTicketsTool, handleSearchTickets)

	addNoteTool := mcp.NewTool("add_note_to_ticket" /* ... */)
	s.AddTool(addNoteTool, handleAddNoteToTicket)

	replyToTicketTool := mcp.NewTool("reply_to_ticket",
		mcp.WithDescription("Reply to a Zammad ticket by email."),
		mcp.WithNumber("ticket_id", mcp.Required(), mcp.Description("The ID of the ticket to reply to.")),
		mcp.WithString("body", mcp.Required(), mcp.Description("The email body/content.")),
		mcp.WithString("to", mcp.Description("The recipient email address(es). If not provided, will use the ticket customer's email.")),
		mcp.WithString("cc", mcp.Description("CC email address(es).")),
		mcp.WithString("subject", mcp.Description("Email subject. If not provided, will use the ticket subject with 'Re:' prefix.")),
		mcp.WithBoolean("internal", mcp.Description("Whether this is an internal email (default: false).")),
	)
	s.AddTool(replyToTicketTool, handleReplyToTicket)

	getTicketTool := mcp.NewTool("get_ticket",
		mcp.WithDescription("Retrieves details for a specific Zammad ticket by its ID."),
		mcp.WithNumber("ticket_id", mcp.Required(), mcp.Description("The ID of the ticket to retrieve.")),
	)
	s.AddTool(getTicketTool, handleGetTicket)

	// --- User Tools --- <--- NEW TOOLS REGISTERED HERE
	getUserTool := mcp.NewTool("get_user",
		mcp.WithDescription("Retrieves details for a specific Zammad user by their ID."),
		mcp.WithNumber("user_id", mcp.Required(), mcp.Description("The ID of the user to retrieve.")),
		mcp.WithBoolean("with_extended_data", mcp.Description("If true, returns all user data including custom fields. If false (default), returns only standard fields.")),
	)
	s.AddTool(getUserTool, handleGetUser) // Register the new handler

	searchUsersTool := mcp.NewTool("search_users",
		mcp.WithDescription("Searches for Zammad users based on a query string (e.g., email, login, name)."),
		mcp.WithString("query", mcp.Required(), mcp.Description("The search query string.")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results. Default: 50."), mcp.DefaultNumber(50)),
	)
	s.AddTool(searchUsersTool, handleSearchUsers) // Register the new handler

	getTicketArticlesTool := mcp.NewTool("get_ticket_articles",
		mcp.WithDescription("Retrieves all articles (communications) for a specific Zammad ticket."),
		mcp.WithNumber("ticket_id", mcp.Required(), mcp.Description("The ID of the ticket whose articles are to be retrieved.")),
	)
	s.AddTool(getTicketArticlesTool, handleGetTicketArticles)

	closeTicketTool := mcp.NewTool("close_ticket",
		mcp.WithDescription("Close a Zammad ticket by setting its state to 'closed'."),
		mcp.WithNumber("ticket_id", mcp.Required(), mcp.Description("The ID of the ticket to close.")),
		mcp.WithString("note", mcp.Description("Optional closing note to add to the ticket.")),
	)
	s.AddTool(closeTicketTool, handleCloseTicket)

	assignTicketTool := mcp.NewTool("assign_ticket",
		mcp.WithDescription("Assign a Zammad ticket to a specific agent user."),
		mcp.WithNumber("ticket_id", mcp.Required(), mcp.Description("The ID of the ticket to assign.")),
		mcp.WithNumber("agent_id", mcp.Required(), mcp.Description("The ID of the agent user to assign the ticket to.")),
		mcp.WithString("note", mcp.Description("Optional note to add when assigning the ticket ** not required **.")),
	)
	s.AddTool(assignTicketTool, handleAssignTicket)

	addTagToTicketTool := mcp.NewTool("add_tag_to_ticket",
		mcp.WithDescription("Add a tag to a Zammad ticket."),
		mcp.WithNumber("ticket_id", mcp.Required(), mcp.Description("The ID of the ticket to add the tag to.")),
		mcp.WithString("tag_name", mcp.Required(), mcp.Description("The name of the tag to add to the ticket.")),
	)
	s.AddTool(addTagToTicketTool, handleAddTagToTicket)

	getTicketTagsTool := mcp.NewTool("get_ticket_tags",
		mcp.WithDescription("Get all tags currently assigned to a specific ticket."),
		mcp.WithNumber("ticket_id", mcp.Required(), mcp.Description("The ID of the ticket to get tags for.")),
	)
	s.AddTool(getTicketTagsTool, handleGetTicketTags)

	listAllTagsTool := mcp.NewTool("list_all_tags",
		mcp.WithDescription("List all tags available in the Zammad system (requires admin.tag permission)."),
	)
	s.AddTool(listAllTagsTool, handleListAllTags)

	searchTagsTool := mcp.NewTool("search_tags",
		mcp.WithDescription("Search for tags by name in the Zammad system."),
		mcp.WithString("search_term", mcp.Required(), mcp.Description("The search term to look for in tag names.")),
	)
	s.AddTool(searchTagsTool, handleSearchTags)

	// Text module tools
	listTextModulesTool := mcp.NewTool("list_text_modules",
		mcp.WithDescription("List all text modules available in the Zammad system."),
	)
	s.AddTool(listTextModulesTool, handleListTextModules)

	getTextModuleTool := mcp.NewTool("get_text_module",
		mcp.WithDescription("Get details of a specific text module by ID."),
		mcp.WithNumber("text_module_id", mcp.Required(), mcp.Description("The ID of the text module to retrieve.")),
	)
	s.AddTool(getTextModuleTool, handleGetTextModule)

	searchTextModulesTool := mcp.NewTool("search_text_modules",
		mcp.WithDescription("Search for text modules by name or content."),
		mcp.WithString("search_term", mcp.Required(), mcp.Description("The search term to look for in text module names or content.")),
	)
	s.AddTool(searchTextModulesTool, handleSearchTextModules)

	// Add create_user, update_user, delete_user tools here if needed
}

// --- Ticket Tool Handlers ---
func handleCreateTicket(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)
	title := mcp.ParseString(request, "title", "")
	group := mcp.ParseString(request, "group", "")
	customer := mcp.ParseString(request, "customer", "")
	body := mcp.ParseString(request, "body", "")
	articleType := mcp.ParseString(request, "type", "note")
	internal := mcp.ParseBoolean(request, "internal", false)
	if title == "" || group == "" || customer == "" || body == "" {
		return mcp.NewToolResultError("Missing required arguments: title, group, customer, body"), nil
	}
	ticket := zammad.Ticket{Title: title, Group: group, Customer: customer, Article: zammad.TicketArticle{Body: body, Type: articleType, Internal: internal}}
	createdTicket, err := zammadClient.TicketCreate(ticket)
	if err != nil {
		log.Printf("Error creating ticket in Zammad: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to create ticket", err), nil
	}
	log.Printf("Successfully created ticket ID %d", createdTicket.ID)
	resultData, _ := json.MarshalIndent(createdTicket, "", "  ")
	return mcp.NewToolResultText(fmt.Sprintf("Ticket created successfully:\n%s", string(resultData))), nil
}

func handleSearchTickets(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)
	query := mcp.ParseString(request, "query", "")
	limit := mcp.ParseInt(request, "limit", 50)
	if query == "" {
		return mcp.NewToolResultError("Missing required argument: query"), nil
	}
	tickets, err := zammadClient.TicketSearch(query, limit)
	if err != nil {
		log.Printf("Error searching tickets in Zammad: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to search tickets", err), nil
	}
	log.Printf("Found %d tickets matching query '%s'", len(tickets), query)
	resultData, err := json.MarshalIndent(tickets, "", "  ")
	if err != nil {
		log.Printf("Error marshalling search results: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to format search results", err), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Search Results (%d found):\n%s", len(tickets), string(resultData))), nil
}

func handleAddNoteToTicket(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)
	ticketID := mcp.ParseInt(request, "ticket_id", 0)
	body := mcp.ParseString(request, "body", "")
	internal := mcp.ParseBoolean(request, "internal", true)
	if ticketID <= 0 || body == "" {
		return mcp.NewToolResultError("Missing or invalid required arguments: ticket_id, body"), nil
	}
	article := zammad.TicketArticle{TicketID: ticketID, Body: body, Type: "note", Internal: internal}
	createdArticle, err := zammadClient.TicketArticleCreate(article)
	if err != nil {
		log.Printf("Error adding note to ticket %d in Zammad: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to add note to ticket %d", ticketID), err), nil
	}
	log.Printf("Successfully added note (Article ID %d) to ticket ID %d", createdArticle.ID, ticketID)
	resultData, _ := json.MarshalIndent(createdArticle, "", "  ")
	return mcp.NewToolResultText(fmt.Sprintf("Note added successfully to ticket %d:\n%s", ticketID, string(resultData))), nil
}

func handleReplyToTicket(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)
	ticketID := mcp.ParseInt(request, "ticket_id", 0)
	body := mcp.ParseString(request, "body", "")
	to := mcp.ParseString(request, "to", "")
	cc := mcp.ParseString(request, "cc", "")
	subject := mcp.ParseString(request, "subject", "")
	internal := mcp.ParseBoolean(request, "internal", false)

	if ticketID <= 0 || body == "" {
		return mcp.NewToolResultError("Missing or invalid required arguments: ticket_id, body"), nil
	}

	// If subject is not provided, fetch the ticket to get its subject
	if subject == "" {
		ticket, err := zammadClient.TicketShow(ticketID)
		if err != nil {
			log.Printf("Error fetching ticket %d to get subject: %v", ticketID, err)
			return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to fetch ticket %d", ticketID), err), nil
		}
		subject = fmt.Sprintf("Re: %s", ticket.Title)
	}

	// Create email article
	article := zammad.TicketArticle{
		TicketID:    ticketID,
		Body:        body,
		Type:        "email",
		Sender:      "Agent",
		Subject:     subject,
		To:          to,
		Cc:          cc,
		Internal:    internal,
		ContentType: "text/html",
	}

	createdArticle, err := zammadClient.TicketArticleCreate(article)
	if err != nil {
		log.Printf("Error sending email reply to ticket %d in Zammad: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to send email reply to ticket %d", ticketID), err), nil
	}

	log.Printf("Successfully sent email reply (Article ID %d) to ticket ID %d", createdArticle.ID, ticketID)
	resultData, _ := json.MarshalIndent(createdArticle, "", "  ")
	return mcp.NewToolResultText(fmt.Sprintf("Email reply sent successfully to ticket %d:\n%s", ticketID, string(resultData))), nil
}

func handleGetTicket(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)
	ticketID := mcp.ParseInt(request, "ticket_id", 0)
	if ticketID <= 0 {
		return mcp.NewToolResultError("Missing or invalid required argument: ticket_id (must be a positive number)"), nil
	}
	ticket, err := zammadClient.TicketShow(ticketID)
	if err != nil {
		log.Printf("Error fetching ticket %d from Zammad via tool: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to get ticket %d", ticketID), err), nil
	}
	log.Printf("Successfully retrieved ticket ID %d via tool", ticketID)
	jsonData, err := json.MarshalIndent(ticket, "", "  ")
	if err != nil {
		log.Printf("Error marshalling ticket %d to JSON (tool): %v", ticketID, err)
		return nil, fmt.Errorf("failed to marshal ticket %d: %w", ticketID, err) // Internal server error
	}
	return mcp.NewToolResultText(fmt.Sprintf("Ticket %d details:\n%s", ticketID, string(jsonData))), nil
}

// --- User Tool Handlers --- <-- NEW HANDLERS

// handleGetUser retrieves details for a specific user by ID using the tool.
func handleGetUser(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	userID := mcp.ParseInt(request, "user_id", 0)
	withExtendedData := mcp.ParseBoolean(request, "with_extended_data", false)

	if userID <= 0 {
		return mcp.NewToolResultError("Missing or invalid required argument: user_id (must be a positive number)"), nil
	}

	var resultData interface{}

	if withExtendedData {
		// For extended data, make a direct HTTP request to get all fields including custom ones
		req, err := zammadClient.NewRequest("GET", fmt.Sprintf("%s/api/v1/users/%d", zammadClient.Url, userID), nil)
		if err != nil {
			log.Printf("Error creating request for user %d: %v", userID, err)
			return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to create request for user %d", userID), err), nil
		}

		// Apply authentication headers manually (matching the client's sendWithAuth logic)
		if zammadClient.Username != "" && zammadClient.Password != "" {
			req.SetBasicAuth(zammadClient.Username, zammadClient.Password)
		}
		if zammadClient.Token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Token token=%s", zammadClient.Token))
		}
		if zammadClient.OAuth != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", zammadClient.OAuth))
		}

		// Make the request
		resp, err := zammadClient.Client.Do(req)
		if err != nil {
			log.Printf("Error fetching extended user data for user %d: %v", userID, err)
			return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to get extended data for user %d", userID), err), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Error response from Zammad for user %d: status %d", userID, resp.StatusCode)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get user %d: HTTP status %d", userID, resp.StatusCode)), nil
		}

		// Decode the full response including custom fields
		var rawUserData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&rawUserData); err != nil {
			log.Printf("Error decoding extended user data for user %d: %v", userID, err)
			return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to decode extended data for user %d", userID), err), nil
		}

		resultData = rawUserData
		log.Printf("Successfully retrieved extended data for user ID %d", userID)
	} else {
		// For standard data, use the client method which returns only defined fields
		user, err := zammadClient.UserShow(userID)
		if err != nil {
			log.Printf("Error fetching user %d from Zammad: %v", userID, err)
			return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to get user %d", userID), err), nil
		}

		// Return only the basic fields
		resultData = map[string]interface{}{
			"id":              user.ID,
			"organization_id": user.OrganizationID,
			"login":           user.Login,
			"firstname":       user.Firstname,
			"lastname":        user.Lastname,
			"email":           user.Email,
			"web":             user.Web,
			"last_login":      user.LastLogin,
		}
		log.Printf("Successfully retrieved standard data for user ID %d", userID)
	}

	jsonData, err := json.MarshalIndent(resultData, "", "  ")
	if err != nil {
		log.Printf("Error marshalling user %d data to JSON: %v", userID, err)
		return nil, fmt.Errorf("failed to marshal user %d data: %w", userID, err)
	}

	dataType := "standard"
	if withExtendedData {
		dataType = "extended"
	}
	return mcp.NewToolResultText(fmt.Sprintf("User %d details (%s data):\n%s", userID, dataType, string(jsonData))), nil
}

// handleSearchUsers searches Zammad users.
func handleSearchUsers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	query := mcp.ParseString(request, "query", "")
	limit := mcp.ParseInt(request, "limit", 50) // Default limit 50

	if query == "" {
		return mcp.NewToolResultError("Missing required argument: query"), nil
	}

	users, err := zammadClient.UserSearch(query, limit)
	if err != nil {
		log.Printf("Error searching users in Zammad: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to search users", err), nil
	}

	log.Printf("Found %d users matching query '%s'", len(users), query)
	resultData, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		log.Printf("Error marshalling user search results: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to format user search results", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("User Search Results (%d found):\n%s", len(users), string(resultData))), nil
}

// --- Add create/update/delete user handlers here if needed ---

// handleGetTicketArticles retrieves all articles for a specific ticket by ID using the tool.
func handleGetTicketArticles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	ticketID := mcp.ParseInt(request, "ticket_id", 0)

	if ticketID <= 0 {
		return mcp.NewToolResultError("Missing or invalid required argument: ticket_id (must be a positive number)"), nil
	}

	articles, err := zammadClient.TicketArticleByTicket(ticketID)
	if err != nil {
		log.Printf("Error fetching articles for ticket %d from Zammad via tool: %v", ticketID, err)
		// Consider if ticket not found should be a specific error
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to get articles for ticket %d", ticketID), err), nil
	}

	log.Printf("Successfully retrieved %d articles for ticket ID %d via tool", len(articles), ticketID)
	jsonData, err := json.MarshalIndent(articles, "", "  ")
	if err != nil {
		log.Printf("Error marshalling articles for ticket %d to JSON (tool): %v", ticketID, err)
		return nil, fmt.Errorf("failed to marshal articles for ticket %d: %w", ticketID, err) // Internal server error
	}

	return mcp.NewToolResultText(fmt.Sprintf("Ticket %d Articles (%d found):\n%s", ticketID, len(articles), string(jsonData))), nil
}

func handleCloseTicket(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	ticketID := mcp.ParseInt(request, "ticket_id", 0)
	note := mcp.ParseString(request, "note", "")

	if ticketID <= 0 {
		return mcp.NewToolResultError("Missing or invalid required argument: ticket_id (must be a positive number)"), nil
	}

	// Update the ticket state to "closed"
	// We need to make a direct API call to avoid sending empty string fields
	// that might cause validation errors in Zammad

	updateData := map[string]interface{}{
		"state": "closed",
	}

	req, err := zammadClient.NewRequest("PUT", fmt.Sprintf("%s/api/v1/tickets/%d", zammadClient.Url, ticketID), updateData)
	if err != nil {
		log.Printf("Error creating request to close ticket %d: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to create request to close ticket %d", ticketID), err), nil
	}

	// Apply authentication headers manually (matching the client's sendWithAuth logic)
	if zammadClient.Username != "" && zammadClient.Password != "" {
		req.SetBasicAuth(zammadClient.Username, zammadClient.Password)
	}
	if zammadClient.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Token token=%s", zammadClient.Token))
	}
	if zammadClient.OAuth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", zammadClient.OAuth))
	}

	// Make the request
	resp, err := zammadClient.Client.Do(req)
	if err != nil {
		log.Printf("Error closing ticket %d: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to close ticket %d", ticketID), err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response from Zammad when closing ticket %d: status %d", ticketID, resp.StatusCode)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to close ticket %d: HTTP status %d", ticketID, resp.StatusCode)), nil
	}

	// Decode the response
	var updatedTicket zammad.Ticket
	if err := json.NewDecoder(resp.Body).Decode(&updatedTicket); err != nil {
		log.Printf("Error decoding response when closing ticket %d: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to decode response when closing ticket %d", ticketID), err), nil
	}

	log.Printf("Successfully closed ticket ID %d", ticketID)

	// If a closing note was provided, add it as an internal note
	if note != "" {
		article := zammad.TicketArticle{
			TicketID: ticketID,
			Body:     note,
			Type:     "note",
			Internal: true,
			Subject:  "Ticket closed",
		}

		createdArticle, err := zammadClient.TicketArticleCreate(article)
		if err != nil {
			log.Printf("Warning: Ticket %d closed successfully, but failed to add closing note: %v", ticketID, err)
			// Don't fail the whole operation if note creation fails
		} else {
			log.Printf("Added closing note (Article ID %d) to ticket ID %d", createdArticle.ID, ticketID)
		}
	}

	resultData, _ := json.MarshalIndent(updatedTicket, "", "  ")
	return mcp.NewToolResultText(fmt.Sprintf("Ticket %d closed successfully:\n%s", ticketID, string(resultData))), nil
}

func handleAssignTicket(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	ticketID := mcp.ParseInt(request, "ticket_id", 0)
	agentID := mcp.ParseInt(request, "agent_id", 0)
	note := mcp.ParseString(request, "note", "")

	if ticketID <= 0 || agentID <= 0 {
		return mcp.NewToolResultError("Missing or invalid required arguments: ticket_id and agent_id (both must be positive numbers)"), nil
	}

	// Update the ticket owner using a direct API call to avoid validation errors
	// We only send the owner_id field that needs to be updated
	updateData := map[string]interface{}{
		"owner_id": agentID,
	}

	req, err := zammadClient.NewRequest("PUT", fmt.Sprintf("%s/api/v1/tickets/%d", zammadClient.Url, ticketID), updateData)
	if err != nil {
		log.Printf("Error creating request to assign ticket %d: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to create request to assign ticket %d", ticketID), err), nil
	}

	// Apply authentication headers manually (matching the client's sendWithAuth logic)
	if zammadClient.Username != "" && zammadClient.Password != "" {
		req.SetBasicAuth(zammadClient.Username, zammadClient.Password)
	}
	if zammadClient.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Token token=%s", zammadClient.Token))
	}
	if zammadClient.OAuth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", zammadClient.OAuth))
	}

	resp, err := zammadClient.Client.Do(req)
	if err != nil {
		log.Printf("Error sending request to assign ticket %d: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to send request to assign ticket %d", ticketID), err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response when assigning ticket %d: Status %d", ticketID, resp.StatusCode)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to assign ticket %d: HTTP %d", ticketID, resp.StatusCode)), nil
	}

	var updatedTicket zammad.Ticket
	if err := json.NewDecoder(resp.Body).Decode(&updatedTicket); err != nil {
		log.Printf("Error decoding response when assigning ticket %d: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to decode response when assigning ticket %d", ticketID), err), nil
	}

	log.Printf("Successfully assigned ticket ID %d to agent ID %d", ticketID, agentID)

	// If a note was provided, add it as an internal note
	if note != "" {
		article := zammad.TicketArticle{
			TicketID: ticketID,
			Body:     note,
			Type:     "note",
			Internal: true,
			Subject:  "Ticket assigned",
		}

		createdArticle, err := zammadClient.TicketArticleCreate(article)
		if err != nil {
			log.Printf("Warning: Ticket %d assigned successfully, but failed to add assignment note: %v", ticketID, err)
			// Don't fail the whole operation if note creation fails
		} else {
			log.Printf("Added assignment note (Article ID %d) to ticket ID %d", createdArticle.ID, ticketID)
		}
	}

	resultData, _ := json.MarshalIndent(updatedTicket, "", "  ")
	return mcp.NewToolResultText(fmt.Sprintf("Ticket %d assigned to agent %d successfully:\n%s", ticketID, agentID, string(resultData))), nil
}

func handleAddTagToTicket(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	ticketID := mcp.ParseInt(request, "ticket_id", 0)
	tagName := mcp.ParseString(request, "tag_name", "")

	if ticketID <= 0 || tagName == "" {
		return mcp.NewToolResultError("Missing or invalid required arguments: ticket_id (must be positive) and tag_name (must not be empty)"), nil
	}

	// Validate tag name - basic validation for common issues
	if len(tagName) > 100 {
		return mcp.NewToolResultError("Tag name is too long (maximum 100 characters)"), nil
	}

	// Log the exact data being sent for debugging
	tagData := map[string]interface{}{
		"object": "Ticket",
		"o_id":   ticketID,
		"item":   tagName,
	}
	log.Printf("Adding tag to ticket - Data: %+v", tagData)

	// Add tag using manual API call with proper authentication
	err := func() error {
		req, err := zammadClient.NewRequest("POST", fmt.Sprintf("%s/api/v1/tags/add", zammadClient.Url), tagData)
		if err != nil {
			return err
		}

		// Apply authentication headers manually but more carefully
		if zammadClient.Username != "" && zammadClient.Password != "" {
			req.SetBasicAuth(zammadClient.Username, zammadClient.Password)
		}
		if zammadClient.Token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Token token=%s", zammadClient.Token))
		}
		if zammadClient.OAuth != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", zammadClient.OAuth))
		}

		// Make the request
		resp, err := zammadClient.Client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Read response body for debugging
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response body: %v", err)
			bodyBytes = []byte("unable to read response")
		}
		bodyString := string(bodyBytes)

		log.Printf("Tag add response: Status %d, Body: %s", resp.StatusCode, bodyString)

		// Zammad API should return 201 Created for successful tag addition
		if resp.StatusCode == http.StatusCreated {
			log.Printf("Successfully added tag '%s' to ticket ID %d (Status: 201)", tagName, ticketID)
			return nil
		} else if resp.StatusCode == http.StatusOK {
			log.Printf("Tag '%s' added to ticket ID %d (Status: 200 - may have already existed)", tagName, ticketID)
			return nil
		} else {
			log.Printf("Error response from Zammad when adding tag '%s' to ticket %d: status %d, body: %s", tagName, ticketID, resp.StatusCode, bodyString)
			return fmt.Errorf("HTTP status %d: %s", resp.StatusCode, bodyString)
		}
	}()

	if err != nil {
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to add tag '%s' to ticket %d", tagName, ticketID), err), nil
	}

	// Optional verification (can be disabled if it causes issues)
	if true { // Set to false to disable verification
		currentTags, err := zammadClient.TicketTagByTicket(ticketID)
		if err != nil {
			log.Printf("Warning: Tag added, but verification failed: %v", err)
			return mcp.NewToolResultText(fmt.Sprintf("Tag '%s' added to ticket %d (verification failed: %v)", tagName, ticketID, err)), nil
		}

		// Check if our tag is in the list
		tagFound := false
		for _, tag := range currentTags {
			if tag.Name == tagName {
				tagFound = true
				break
			}
		}

		if tagFound {
			return mcp.NewToolResultText(fmt.Sprintf("Tag '%s' successfully added to ticket %d (verified)", tagName, ticketID)), nil
		} else {
			log.Printf("Warning: Tag '%s' was added to ticket %d but not found in verification list", tagName, ticketID)
			return mcp.NewToolResultText(fmt.Sprintf("Tag '%s' added to ticket %d (not found in verification - may need time to propagate)", tagName, ticketID)), nil
		}
	} else {
		return mcp.NewToolResultText(fmt.Sprintf("Tag '%s' added to ticket %d", tagName, ticketID)), nil
	}
}

// handleGetTicketTags retrieves all tags for a specific ticket
func handleGetTicketTags(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	ticketID := mcp.ParseInt(request, "ticket_id", 0)
	if ticketID <= 0 {
		return mcp.NewToolResultError("Missing or invalid required argument: ticket_id (must be positive)"), nil
	}

	tags, err := zammadClient.TicketTagByTicket(ticketID)
	if err != nil {
		log.Printf("Error fetching tags for ticket %d: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to get tags for ticket %d", ticketID), err), nil
	}

	if len(tags) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No tags found for ticket %d", ticketID)), nil
	}

	resultData, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		log.Printf("Error marshalling tags for ticket %d: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr("Failed to format tag results", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Tags for ticket %d (%d found):\n%s", ticketID, len(tags), string(resultData))), nil
}

// handleListAllTags lists all tags in the Zammad system
func handleListAllTags(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	tags, err := zammadClient.TagAdminList()
	if err != nil {
		log.Printf("Error fetching all tags: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to list all tags", err), nil
	}

	if len(tags) == 0 {
		return mcp.NewToolResultText("No tags found in the system"), nil
	}

	resultData, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		log.Printf("Error marshalling all tags: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to format tag list", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("All tags in system (%d found):\n%s", len(tags), string(resultData))), nil
}

// handleSearchTags searches for tags by name
func handleSearchTags(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	searchTerm := mcp.ParseString(request, "search_term", "")
	if searchTerm == "" {
		return mcp.NewToolResultError("Missing required argument: search_term"), nil
	}

	tags, err := zammadClient.TagSearch(searchTerm)
	if err != nil {
		log.Printf("Error searching tags with term '%s': %v", searchTerm, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to search tags with term '%s'", searchTerm), err), nil
	}

	if len(tags) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No tags found matching search term '%s'", searchTerm)), nil
	}

	resultData, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		log.Printf("Error marshalling tag search results: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to format tag search results", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Tags matching '%s' (%d found):\n%s", searchTerm, len(tags), string(resultData))), nil
}

// Text Module handlers

// handleListTextModules retrieves all text modules
func handleListTextModules(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	req, err := zammadClient.NewRequest("GET", fmt.Sprintf("%s/api/v1/text_modules", zammadClient.Url), nil)
	if err != nil {
		log.Printf("Error creating request for text modules: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to create request for text modules", err), nil
	}

	// Apply authentication headers
	if zammadClient.Username != "" && zammadClient.Password != "" {
		req.SetBasicAuth(zammadClient.Username, zammadClient.Password)
	}
	if zammadClient.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Token token=%s", zammadClient.Token))
	}
	if zammadClient.OAuth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", zammadClient.OAuth))
	}

	resp, err := zammadClient.Client.Do(req)
	if err != nil {
		log.Printf("Error fetching text modules: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to fetch text modules", err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response when fetching text modules: Status %d", resp.StatusCode)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch text modules: HTTP %d", resp.StatusCode)), nil
	}

	var textModules []TextModule
	if err := json.NewDecoder(resp.Body).Decode(&textModules); err != nil {
		log.Printf("Error decoding text modules response: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to decode text modules response", err), nil
	}

	if len(textModules) == 0 {
		return mcp.NewToolResultText("No text modules found"), nil
	}

	resultData, err := json.MarshalIndent(textModules, "", "  ")
	if err != nil {
		log.Printf("Error marshalling text modules: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to format text modules", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Text modules (%d found):\n%s", len(textModules), string(resultData))), nil
}

// handleGetTextModule retrieves a specific text module by ID
func handleGetTextModule(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	textModuleID := mcp.ParseInt(request, "text_module_id", 0)
	if textModuleID <= 0 {
		return mcp.NewToolResultError("Missing or invalid required argument: text_module_id (must be positive)"), nil
	}

	req, err := zammadClient.NewRequest("GET", fmt.Sprintf("%s/api/v1/text_modules/%d", zammadClient.Url, textModuleID), nil)
	if err != nil {
		log.Printf("Error creating request for text module %d: %v", textModuleID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to create request for text module %d", textModuleID), err), nil
	}

	// Apply authentication headers
	if zammadClient.Username != "" && zammadClient.Password != "" {
		req.SetBasicAuth(zammadClient.Username, zammadClient.Password)
	}
	if zammadClient.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Token token=%s", zammadClient.Token))
	}
	if zammadClient.OAuth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", zammadClient.OAuth))
	}

	resp, err := zammadClient.Client.Do(req)
	if err != nil {
		log.Printf("Error fetching text module %d: %v", textModuleID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to fetch text module %d", textModuleID), err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return mcp.NewToolResultError(fmt.Sprintf("Text module %d not found", textModuleID)), nil
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response when fetching text module %d: Status %d", textModuleID, resp.StatusCode)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch text module %d: HTTP %d", textModuleID, resp.StatusCode)), nil
	}

	var textModule TextModule
	if err := json.NewDecoder(resp.Body).Decode(&textModule); err != nil {
		log.Printf("Error decoding text module %d response: %v", textModuleID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to decode text module %d response", textModuleID), err), nil
	}

	resultData, err := json.MarshalIndent(textModule, "", "  ")
	if err != nil {
		log.Printf("Error marshalling text module %d: %v", textModuleID, err)
		return mcp.NewToolResultErrorFromErr("Failed to format text module", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Text module %d:\n%s", textModuleID, string(resultData))), nil
}

// handleSearchTextModules searches for text modules by name or content
func handleSearchTextModules(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Handling tool call: %s", request.Params.Name)

	searchTerm := mcp.ParseString(request, "search_term", "")
	if searchTerm == "" {
		return mcp.NewToolResultError("Missing required argument: search_term"), nil
	}

	// Get all text modules first, then filter them
	req, err := zammadClient.NewRequest("GET", fmt.Sprintf("%s/api/v1/text_modules", zammadClient.Url), nil)
	if err != nil {
		log.Printf("Error creating request for text modules search: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to create request for text modules search", err), nil
	}

	// Apply authentication headers
	if zammadClient.Username != "" && zammadClient.Password != "" {
		req.SetBasicAuth(zammadClient.Username, zammadClient.Password)
	}
	if zammadClient.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Token token=%s", zammadClient.Token))
	}
	if zammadClient.OAuth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", zammadClient.OAuth))
	}

	resp, err := zammadClient.Client.Do(req)
	if err != nil {
		log.Printf("Error fetching text modules for search: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to fetch text modules for search", err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response when fetching text modules for search: Status %d", resp.StatusCode)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch text modules for search: HTTP %d", resp.StatusCode)), nil
	}

	var allTextModules []TextModule
	if err := json.NewDecoder(resp.Body).Decode(&allTextModules); err != nil {
		log.Printf("Error decoding text modules search response: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to decode text modules search response", err), nil
	}

	// Filter text modules by search term (case-insensitive search in name, keywords, and content)
	var matchingModules []TextModule

	for _, module := range allTextModules {
		if containsIgnoreCase(module.Name, searchTerm) ||
			containsIgnoreCase(module.Keywords, searchTerm) ||
			containsIgnoreCase(module.Content, searchTerm) {
			matchingModules = append(matchingModules, module)
		}
	}

	if len(matchingModules) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No text modules found matching search term '%s'", searchTerm)), nil
	}

	resultData, err := json.MarshalIndent(matchingModules, "", "  ")
	if err != nil {
		log.Printf("Error marshalling text modules search results: %v", err)
		return mcp.NewToolResultErrorFromErr("Failed to format text modules search results", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Text modules matching '%s' (%d found):\n%s", searchTerm, len(matchingModules), string(resultData))), nil
}

// Helper function for case-insensitive string contains
func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	// Simple case-insensitive search
	sLower := toLower(s)
	substrLower := toLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// Simple toLower function for ASCII characters
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			result[i] = s[i] + 32
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}
