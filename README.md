## Books

pg:
```bash
docker run --name books-pg -e POSTGRES_PASSWORD=123456Pg -v /data/volumes/pg-books:/var/lib/postgresql/data -p 5432:5432 -d postgres
```

jwt:
```bash
curl -d '{"username": "admin", "password": "admin"}' -H "Content-Type:application/json" http://localhost:18080/login
curl -H "Authorization:Bearer TOKEN_RETURNED_FROM_ABOVE" http://localhost:18080/auth_test
curl -H "Authorization:Bearer TOKEN_RETURNED_FROM_ABOVE" http://localhost:18080/refresh_token
```

#### Thanks
https://github.com/ant0ine
