# 🏃‍ Job Runner

A prototype job worker service that provides a gRPC API to run arbitrary processes on Linux or Darwin hosts.

**Features:**

- RPCs to start, stop, and query a process.
- Stream RPC to follow logs of a process (supports multiple concurrent clients).
- Clients authenticated with mTLS.
- Access control list authorization.

## 🚀 Running

Before proceeding, please ensure you have Docker installed and running.

1. Generate certificates using `cfssl`
   ([bundled as a module dependency](https://play-with-go.dev/tools-as-dependencies_go115_en)).

   ```shell
   make gencert
   ```

3. Build the `jobrunner` image.

   ```shell
   make build
   ```

4. Run the `jobrunner` gRPC service.

   ```shell
   make run
   ```

5. Make requests to `localhost:9090` using a gRPC client or simply run the integration test using `make integration`.

## 🔬 Testing

- Unit tests: `make unit`
- Integration tests (server must be running): `make integration`
