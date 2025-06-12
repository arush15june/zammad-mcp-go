package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		server.WithInstructions("This server provides access to Zammad tickets and users via resources and tools (e.g., create_ticket, get_ticket, search_tickets, get_user, search_users)."),
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
		mcp.WithString("note", mcp.Description("Optional note to add when assigning the ticket.")),
	)
	s.AddTool(assignTicketTool, handleAssignTicket)

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
	ticket := zammad.Ticket{
		State: "closed",
	}

	updatedTicket, err := zammadClient.TicketUpdate(ticketID, ticket)
	if err != nil {
		log.Printf("Error closing ticket %d in Zammad: %v", ticketID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to close ticket %d", ticketID), err), nil
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

	// Update the ticket to assign it to the specified agent
	ticket := zammad.Ticket{
		OwnerID: agentID,
	}

	updatedTicket, err := zammadClient.TicketUpdate(ticketID, ticket)
	if err != nil {
		log.Printf("Error assigning ticket %d to agent %d in Zammad: %v", ticketID, agentID, err)
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("Failed to assign ticket %d to agent %d", ticketID, agentID), err), nil
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
