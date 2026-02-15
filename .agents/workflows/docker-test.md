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

5. Run the Bash integration test suite:
```bash
./docker-compose-test.sh
```

6. (Optional) Run the Python integration test suite:
```bash
python docker-compose-test.py
```

7. (Optional) Run API endpoint tests:
```bash
./test_proxy_api.sh
```

8. View container logs for any errors:
```bash
docker compose -f compose-test.yml logs --tail=50 tailrelay-test
```

9. Cleanup test environment:
```bash
docker compose -f compose-test.yml down
```
