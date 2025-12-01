# kairos

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Tor](https://img.shields.io/badge/Tor-7D4698?style=for-the-badge&logo=Tor-Browser&logoColor=white)
![Linux](https://img.shields.io/badge/Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black)
![BoltDB](https://img.shields.io/badge/BoltDB-BB3333?style=for-the-badge&logo=google-cloud&logoColor=white)
![Drand](https://img.shields.io/badge/Drand-141d26?style=for-the-badge&logo=lock&logoColor=white)
![Reed-Solomon](https://img.shields.io/badge/Reed--Solomon-blueviolet?style=for-the-badge)
![Shamir](https://img.shields.io/badge/Shamir_Secret_Sharing-ff69b4?style=for-the-badge)
![License](https://img.shields.io/badge/License-GNU_AGPL_v3-blue?style=for-the-badge&logo=gnu&logoColor=white)
![Status](https://img.shields.io/badge/Proof_of_Concept-orange?style=for-the-badge)

`kairos` is an anonymous, decentralized peer-to-peer (P2P) file-sharing system. It is designed to provide maximum privacy by combining the anonymity of the Tor network with a secure, resilient storage architecture.

Its unique feature is its `time-lock` mechanism, powered by **Drand**. By leveraging Drand's distributed time-lock encryption, uploaded files remain cryptographically sealed and inaccessible until a specific release date and time has passed. This ensures that data decryption is mathematically impossible before the predetermined moment, guaranteeing a secure release without relying on the honesty of the storage nodes..


> [!WARNING]
> This project is currently a Proof of Concept (POC) and is not suitable for a production environment. It is intended for developer use only.
> Any use for illegal purposes is neither supported nor encouraged by the developers of this project.

---



## Table of Contents



* [Key Features](#key-features)

* [System Architecture](#system-architecture)

    * [The Server](#the-server)

    * [The Client and Cli](#the-client-and-cli)

    * [Communication Flow](#communication-flow)

* [Technology Stack](#technology-stack)

* [Installation and Setup](#installation-and-setup)

    * [Prerequisites](#prerequisites)

    * [1. Server Setup](#1-server-setup)

    * [2. Client Setup](#2-client-setup)
      
    * [3. CLI Setup](#3-cli-setup)

* [Security Considerations](#security-considerations)



---



## Key Features



* **Complete Anonymity via Tor**: All network communication, including peer discovery with bootstrap servers and direct P2P file transfers, is routed through the Tor network. Clients and servers operate as .onion hidden services, ensuring no real IP addresses are ever exposed or logged.

* **Time-Lock Release Mechanism**: Users must specify a release-time flag (e.g., 2025-12-01T15:00:00Z) when uploading a file. Unlike traditional systems that rely on server trust, Kairos uses Drand time-lock encryption. The block's decryption keys are cryptographically sealed against a future "round" of the Drand beacon network. This ensures that the content remains mathematically inaccessible to everyone (including the storage nodes) until the Drand network publishes the randomness for that specific time, guaranteeing a secure and trustless release.

* **End-to-End File Encryption**: Before being fragmented, each block of the original file is individually encrypted using AES-GCM with a unique, randomly generated key. This ensures the file content remains completely unreadable to the peers storing it.

* **Decentralized Key Management**: Each AES encryption key for each block is split into multiple parts using Shamir's Secret Sharing. These key parts are distributed across different nodes along with the data shards, meaning no single node ever holds both a piece of data and the full key required to decrypt it.

* **Data Resiliency (Reed-Solomon)**: The encrypted file blocks are fragmented using Reed-Solomon erasure coding, splitting them into data shards and parity shards. This allows the original file to be fully reconstructed even if multiple nodes or data chunks are lost or unavailable.

* **Embedded Storage**: All data  is stored using Bolt DB, an embedded key/value local storage on client and server side.

* **Strong Authentication**: All critical network actions, such as uploading a file manifest or requesting a chunk, are protected by Ed25519 digital signatures. This verifies the sender's identity and ensures the integrity of the request.

* **Simple CLI Interface**: Users interact with the network via a straightforward command-line interface. Key commands include **start** to add the local client to the network as Kairos node, **put** to upload a file through different nodes and set its release-time and **get** to download and reconstruct a file using its unique ID.



---



## System Architecture



The system is split into two main components:

### The Server

This is a lightweight Go server that acts as an anonymous "phone book" and manifest host.

* It runs as its own Tor Onion Service (.onion).

* Its purpose is to maintain a list of active peers (nodes) and store the FileManifest (metadata) for files in the network.
  
* Its synchronizes its data with the other Bootstrap Servers periodically and delete the old data (manifest files and active users) from the database after a desired time.

* **It never handles or sees any actual file chunks.**

* All critical endpoints (like subscribing a node or uploading a manifest) are protected by Ed25519 digital signatures to verify the peer's identity.

* It uses an embedded BoltDB to store active node lists and file manifests.

### The Client and Cli

This is the application the user runs. It is a hybrid app consisting of a Go backend and a command-line interface (CLI).

#### The Backend (`/client`): 

This single Go process serves two roles:

* **Local API**: It runs on localhost to serve the CLI and handle user actions (like put or get).

* **Onion Peer API**: When started via the start command, it spawns its own Tor process to create a public .onion address. It listens on this address to receive chunks (SaveChunk) and serve chunks (GetChunk) to other peers.

### The Command-Line Interface (`/cli`): 

A simple interface used to send commands to the local backend (`start`, `put` and `get`).

* The client's backend spawns its own Tor process to become a fully anonymous peer in the network.
  
* All outgoing communication is forcibly routed through the local Tor SOCKS proxy.



### Communication Flow



#### Upload Flow (Client A puts a file)

* **Client A** (the uploader) starts the node (kairos `start`), creates its .onion service and registers itself with a **Bootstrap Server**. Other peers (Clients B, C, D...) do the same.
* **Client A** runs kairos `put` with file-path and release-time flags.
* **Client A**'s backend generates a unique random AES key for each file block, encrypts the block, and fragments it using Reed-Solomon. The key is first encrypted using Drand time-lock encryption, targeting the specific beacon round corresponding to the release_time. This time-locked key is then split using Shamir's Secret Sharing, with each key fragment being paired with a specific data chunk
* **Client A** contacts the `Bootstrap Server` to request a list of active nodes (e.g. it receives Clients B, C, D).
* **Client A** generates a FileManifest mapping which chunk will go to which peer (e.g. chunk 1 -> Client B, chunk 2 -> Client C...) and sets the release_time. This manifest does not contain chunk data.
* **Client A** signs and uploads this FileManifest to the `Bootstrap Server`. The server stores it.
* **Client A** connects directly to each node (Client B, C, D...) at their .onion addresses and uploads their respective data chunk, key part and the release_time.

The peers (B, C, D) receive their chunks and save them to their local BoltDB to hold.

#### Download Flow (Client Z gets the file)

* **Client Z** (the downloader) knows the FileId for the file.
* **Client Z** runs kairos `get` --file-id="...".
* **Client Z** contacts the `Bootstrap Server` and requests the FileManifest using the FileId.
* **The Bootstrap Server** finds the manifest and sends it to `Client Z`. `The Bootstrap Server` is no longer involved.
* **Client Z** reads the manifest and sees it needs chunks from Clients B, C, D...
* **Client Z** connects directly to `Client B` at its .onion address and requests chunk 1 of block 1.
* **Client B** it sends chunk 1 to `Client Z`.
* **Client Z** repeats this process for all necessary chunks for the block 1 with Clients C, D, etc.
* Once **Client Z** has collected enough data chunks and key fragments, it first reconstructs the time-locked keys using Shamir's Secret Sharing. It then decrypts these keys using Drand (by fetching the randomness for the specific round). Only after revealing the valid AES key does it reconstruct the data blocks (Reed-Solomon), decrypt them and assemble the original file locally.



---



## Technology Stack



* **Core Language**: Go (Golang)

* **Command-Line Interface**: github.com/spf13/cobra

* **Database**: BoltDB (github.com/boltdb/bolt)

* **Network & Proxy**:

  1. **Tor** (managed as an os/exec process)
  2. **golang.org/x/net/proxy** (for the SOCKS5 proxy client)
  3. **Go native net/http** (for local and peer APIs)

* **Cryptography**:
  
  1. **Go native crypto module** (for crypto/ed25519 and crypto/aes)
  2. **github.com/klauspost/reedsolomon** (for Reed-Solomon coding)
  3. **github.com/corvus-ch/shamir** (for Shamir's Secret Sharing)
  4. **github.com/drand/tlock** (for Drand Time-lock encryption)
---



## Installation and Setup



This project requires manual configuration to set up the Tor services.


> [!WARNING]
> ### Prerequisites
>
>
>
> * Go (v 1.24 or later)
>
> * **Tor Expert Bundle:** You **must** download the correct Tor Expert Bundle for your OS (Linux is the only supported for now) from the [official Tor Project website](https://www.torproject.org/download/expert/).



### 1. Server Setup



The Bootstrap Server MUST be set up first, as its onion address is required by the clients and other servers.



1.  **Clone the repository:**

    ```bash

    git clone https://github.com/FraMan97/kairos.git

    cd kairos/server

    ```



2.  **Install dependencies:**

    ```bash

    go mod tidy

    ```



3.  **Configure Tor:**

    * Extract the **Tor Expert Bundle** you downloaded into the `server/internal` directory.

    * Rename the folder to `tor-bundle-default`.




4.  **Start the server (with flag --bootstrap-servers=`6smhzrvdwljwlyaov7lqi7w5m6gzbcqtcyvo6mjkco47beou7ucafyyd.onion:3000`,`6smhzrvdwljwlyaov7lqi7w5mkslbcqtcyvo6mjkco47beou7ucafyyd.onion:3001`,.. or --no-bootstrap-servers):**

    ```bash

    cd cmd/k-server

    go run . [--bootstrap-servers or --no-bootstrap-servers]

    ```
   


5.  **Get the Server's Onion Address:**

    * After starting, the public_key and private_key of Tor are generated in the home directory (`~/.kairos/server/keys`)



6.  **Database file generation:**

    * After starting, a file `kairos_boltdb.db` is generated in the home directory (`~/.kairos/server/database`)



### 2. Client Setup

1.  **Clone the repository:**

    ```bash

    git clone https://github.com/FraMan97/kairos.git

    ```

2.  **Open a new terminal and navigate to the `client` directory:**

    ```bash

    cd kairos/client

    ```



3.  **Install dependencies:**

    ```bash

    go mod tidy

    ```



3.  **Configure Tor (same as server):**

    * Extract the **Tor Expert Bundle** into the `client/internal` directory.

    * Rename the folder to `tor-bundle-default`.



4.  **Configure the destination directory which will contain the downlodable files:**

    * In the file `client/internal/config/config.go` change the variable `FileGetDestDir`



5.  **Build and start the client (use the flag --bootstrap-server and set with the bootstrap servers .onion address including the port `6smhzrvdwljwlyaov7lqi7w5m6gzbcqtcyvo6mjkco47beou7ucafyyd.onion:3000`):**

    ```bash

    cd cmd/k-client
    go run . [--bootstrap-servers or --no-bootstrap-servers]

    ```



6.  **Database file generation:**

    * After starting, a file `kairos_boltdb.db` is generated in the home directory (`~/.kairos/client/database`)



### 3. CLI Setup



1.  **Open a new terminal and navigate to the `cli` directory:**

    ```bash

    cd kairos/cli

    ```



2.  **Install dependencies:**

    ```bash

    go mod tidy

    ```



3.  **Link CLI to Client:**

    * Open the `cli/config/config.go` file.

    * Paste the client's `port` into the `Port` variable. 

        ```config.go

        Port=8081

        ```



3.  **Launch different commands:**

    ```bash

    go run . start
    go run . put --file-path=/path/to/file --release-time=2025-12-01T15:00:00Z
    go run . get --fileId=mahdska...

    ```


4.  **Get the Client's Onion Address:**

    * After the `start` command, Tor will generate the hidden service and the public_key and private_key in the home directory (`~/.kairos/client/keys`)


---



## Security Considerations



This project was designed with security and anonymity as the highest priorities.



* **No IP Logging**: The Tor Onion Service architecture prevents nodes and bootstrap servers from ever knowing each other's real IP addresses.

* **Multi-Layered Encryption**: Files are end-to-end encrypted on the uploader's machine using AES-GCM before upload. The decryption key itself is then split using Shamir's Secret Sharing and distributed, ensuring no peer node can read the data it stores.

* **Strong Authentication**: All critical network actions (like uploading manifests or subscribing as a node) are verified using Ed25519 digital signatures. This prevents spoofing and ensures an attacker cannot impersonate a peer just by knowing their .onion address.

* **CLI-Backend Separation**: The user-facing kairos CLI (/cli) only sends HTTP commands to the local client backend running on localhost. All sensitive cryptographic material (like the Ed25519 private key) is managed by the backend process, not the CLI tool.

* **Secure Storage API**: The peer-facing API for retrieving data (GetChunk) is not vulnerable to path traversal attacks. It serves data based on a unique chunkId (UUID) from an embedded BoltDB database, not by reading arbitrary paths from the filesystem.

* **Trustless Time-Lock**: The enforcement of the release time relies on Drand's distributed randomness beacon, not on the honesty of the storage nodes. The block encryption keys are cryptographically sealed against a specific future Drand round. This guarantees that decryption is mathematically impossible before the release time, even if the storage nodes collude or are compromised.



---
