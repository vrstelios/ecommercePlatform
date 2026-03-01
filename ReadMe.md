product-service/
├── main.go             # Entry point, setup router & config
├── handler/            # HTTP Handlers (εκεί που δέχεσαι το GET /products)
├── repository/         # Εδώ θα μπει ο κώδικας για το Elasticsearch
├── models/             # Τα structs (Product, Category, κλπ)
└── config/             # Config για ports, ES URLs κλπ