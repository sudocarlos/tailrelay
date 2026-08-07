---
description: Run the full Docker integration test suite (build image, start compose, run tests, cleanup)
---

## Docker Integration Test Workflow

// turbo-all

1. Build the frontend assets:
```bash
make frontend-build
```

2. Build the development Docker image:
```bash
make dev-docker-build
```

3. Start the test environment:
```bash
docker compose -f compose-test.yml up -d
```

4. Wait for services to initialize:
```bash
sleep 5
```

5. Run the Python integration test suite (the canonical integration entrypoint):
```bash
pytest tests/integration/ -v
```

6. (Optional) Run the Bash integration smoke tests:
```bash
./docker-compose-test.sh
```

7. View container logs for any errors:
```bash
docker compose -f compose-test.yml logs --tail=50 tailrelay-test
```

8. Cleanup test environment:
```bash
docker compose -f compose-test.yml down
```
