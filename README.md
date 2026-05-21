# Infinite Experiment Backend

Infinite Experiment Backend is a Go-based application designed to serve requests for a Discord bot. It provides a hybrid API approach using both REST and gRPC endpoints to handle different use cases. This repository is structured to support fast, iterative local development with hot reloading (via Air) and production-ready builds using Docker multi-stage builds.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Getting Started](#getting-started)
  - [Local Development](#local-development)
  - [Production Build](#production-build)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Overview

This project serves as the backend for a Discord bot, managing communication between the bot and services such as a PostgreSQL database, REST endpoints, and gRPC endpoints for real-time communication. The goal is to provide a fast and scalable system that can be easily tested locally and deployed in production.

## Features

- **Hybrid API Design:**  
  Combines REST for simple request-response operations with gRPC for performance-critical or streaming functionalities.

- **Dockerized Development & Production:**  
  Uses a multi-stage Dockerfile to provide separate configurations for local development (with hot reloading via Air) and production builds.

- **Environment Configuration:**  
  Easily switch between local (`.env.local`) and production (`.env.production`) settings.

- **Vizburo UI Styling:**
  Active Vizburo templates use hand-authored design-system CSS from `static/css/design-system.css`. Tailwind is not part of the active root Vizburo build path; edit the design-system stylesheet directly and rely on the versioned `/static/css/design-system.css?v=...` URL for cache refreshes.

## Getting Started

### Local Development

To start local development with hot reloading:

1. **Ensure Docker is installed and running.**
2. **Configure your local environment variables:**  
   Create a `.env.local` file in the root directory with variables similar to:
   ```env
   APP_ENV=local
   DEBUG=true
   PORT=8080
   ```
3. **Run the application using Docker Compose:**
    `docker-compose -f docker-compose.local.yml up --build`
4. **Stop application using Docker Compose:**
    `docker componse -f docker-compose.local.yml down`

5. **To rebuild a single service:**
    `docker-compose -f docker-compose.prod.yml up -d --build api`
    
### Production Build

To build a production-ready image:

1. **Configure your production environment variables:**
    Create a `.env.production` file in the root directory with variables like:
    ```env
    APP_ENV=production
    DEBUG=false
    PORT=8080
    ```
2. **Build the production image using Docker:**

    `docker build --target prod -t politburo:latest .`

3. **Deploy the production container:**

    docker run -p 8080:8080 politburo:latest

    Alternatively, push this image to a container registry and deploy it on your target VM or cloud platform.
