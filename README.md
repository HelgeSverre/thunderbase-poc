# 🌩️ ThunderBase - Real-time Backend with SQLite and WebSockets

<center>
<img src="./art/logo.svg" width="200" alt="ThunderBase Logo">
</center>

## Proof of Concept

ThunderBase is a proof-of-concept application that demonstrates real-time database subscriptions using SQLite,
WebSockets, and database triggers. This project explores the possibility of creating a lightweight, real-time database
solution similar to Firebase or PocketBase.

### Features

- SQLite backend for data storage
- WebSocket connections for real-time updates
- Database triggers for change detection
- Automatic broadcasting of database changes to connected clients

## How It Works

1. **SQLite Database**: ThunderBase uses SQLite as its primary data store.
2. **Change Detection**: Database triggers are automatically created for each table to detect INSERT, UPDATE, and DELETE
   operations.
3. **WebSocket Server**: A WebSocket server is set up to handle client connections.
4. **Real-time Updates**: When a change occurs in the database, it's broadcasted to all connected clients in real-time.

## Demo

[![Demo Video](https://brief.cleanshot.cloud/media/70425/OdtPa6rcDv0BevDyxhPxcUnOAagbAl4fuObaOPPR.mp4)](https://share.helgesver.re/2wHQCWpm)

## Getting Started

1. Clone the repository
2. Run `go mod tidy` to install dependencies
3. Start the server: `go run main.go`
4. Open `http://localhost:8080` in your browser to see the demo

## API

- `/ws`: WebSocket endpoint for real-time updates
- `/insert`: HTTP endpoint to insert dummy data (for testing)

## Future Exploration

This proof of concept is just the beginning. Future areas of exploration include:

- Authentication and authorization
- Data validation and schema enforcement
- Query API for more complex data operations
- Scaling considerations for larger datasets and more concurrent users
- Offline support and data syncing

## Motivation

The goal of this project is to understand the challenges and complexities involved in building a real-time database
system. By creating a simplified version, we can:

1. Identify the core components required for such a system
2. Understand the performance implications of different design choices
3. Explore the limitations of using SQLite for real-time applications
4. Gain insights into the architecture of more complex systems like Firebase or PocketBase

## Feedback Welcome

This is an early-stage proof of concept, and I'm excited to hear what you think! Feel free to open issues, suggest
improvements, or share your thoughts on the approach.

## Installation

```shell
# Clone the repository
git clone git@github.com:HelgeSverre/thunderbase-poc.git
cd thunderbase-poc

# Install dependencies
go mod tidy

# Start the server
go run thunderbase
```

## Formatting

```shell
npx prettier *.html *.js README.md --write
```
