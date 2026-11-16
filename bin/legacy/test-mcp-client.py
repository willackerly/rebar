#!/usr/bin/env python3
"""test-mcp-client — Test client for the ASK MCP server.

Demonstrates the full MCP protocol implementation.
"""

import json
import requests
import sys

def send_mcp_request(server_url, request):
    """Send an MCP JSON-RPC request to the server."""
    response = requests.post(f"{server_url}/mcp", json=request)
    response.raise_for_status()
    return response.json()

def main():
    if len(sys.argv) != 2:
        print("Usage: test-mcp-client <server-url>")
        print("Example: test-mcp-client http://localhost:7232")
        sys.exit(1)

    server_url = sys.argv[1]

    print("=== MCP Client Test ===\n")

    # Step 1: Initialize
    print("1. Initializing MCP session...")
    init_response = send_mcp_request(server_url, {
        "jsonrpc": "2.0",
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "test-client", "version": "1.0.0"}
        },
        "id": 1
    })
    print(f"   Server: {init_response['result']['serverInfo']['name']}")
    print(f"   Protocol: {init_response['result']['protocolVersion']}")
    print()

    # Step 2: List tools
    print("2. Listing available tools...")
    tools_response = send_mcp_request(server_url, {
        "jsonrpc": "2.0",
        "method": "tools/list",
        "params": {},
        "id": 2
    })

    tools = tools_response["result"]["tools"]
    print(f"   Found {len(tools)} agent tools:")
    for i, tool in enumerate(tools[:3], 1):  # Show first 3 for brevity
        print(f"   {i}. {tool['name']}: {tool['description']}")
    if len(tools) > 3:
        print(f"   ... and {len(tools) - 3} more")
    print()

    # Step 3: List resources
    print("3. Listing available resources...")
    resources_response = send_mcp_request(server_url, {
        "jsonrpc": "2.0",
        "method": "resources/list",
        "params": {},
        "id": 3
    })

    resources = resources_response["result"]["resources"]
    print(f"   Found {len(resources)} agent resources:")
    for i, resource in enumerate(resources[:3], 1):  # Show first 3
        print(f"   {i}. {resource['name']} ({resource['uri']})")
    if len(resources) > 3:
        print(f"   ... and {len(resources) - 3} more")
    print()

    # Step 4: Call a tool (if any available)
    if tools:
        print("4. Calling an agent tool...")
        first_tool = tools[0]
        print(f"   Using tool: {first_tool['name']}")

        tool_response = send_mcp_request(server_url, {
            "jsonrpc": "2.0",
            "method": "tools/call",
            "params": {
                "name": first_tool["name"],
                "arguments": {
                    "question": "What is your role and purpose?",
                    "verbose": False
                }
            },
            "id": 4
        })

        if "result" in tool_response:
            content = tool_response["result"]["content"][0]["text"]
            print(f"   Agent response: {content[:200]}...")
        else:
            print(f"   Error: {tool_response.get('error', {}).get('message', 'Unknown error')}")
        print()

    # Step 5: Read a resource (if any available)
    if resources:
        print("5. Reading an agent resource...")
        first_resource = resources[0]
        print(f"   Reading: {first_resource['name']}")

        resource_response = send_mcp_request(server_url, {
            "jsonrpc": "2.0",
            "method": "resources/read",
            "params": {
                "uri": first_resource["uri"]
            },
            "id": 5
        })

        if "result" in resource_response:
            content = resource_response["result"]["contents"][0]["text"]
            print(f"   Content: {content[:200]}...")
        else:
            print(f"   Error: {resource_response.get('error', {}).get('message', 'Unknown error')}")
        print()

    print("=== MCP Test Complete ===")

if __name__ == "__main__":
    main()