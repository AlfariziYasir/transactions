# Variabel Path
PROTO_USER_DIR = proto/user
PROTO_ORDER_DIR = proto/order
PROTO_INVENTORY_DIR = proto/inventory
THIRD_PARTY = ../third_party
OUT_DIR = .

user-proto:
	mkdir -p docs
	
	protoc -I . \
		-I $(THIRD_PARTY) \
		--go_out=$(OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(OUT_DIR) --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(OUT_DIR) --grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=./docs --openapiv2_opt=logtostderr=true,allow_merge=true,merge_file_name=user_api \
		$(PROTO_USER_DIR)/user.proto

	cp docs/user_api.swagger.json ../services/user/cmd/user_api.swagger.json
.PHONY: user-proto

order-proto:
	mkdir -p docs
	
	protoc -I . \
		-I $(THIRD_PARTY) \
		--go_out=$(OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(OUT_DIR) --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(OUT_DIR) --grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=./docs --openapiv2_opt=logtostderr=true,allow_merge=true,merge_file_name=order_api \
		$(PROTO_ORDER_DIR)/order.proto

	cp docs/order_api.swagger.json ../services/order/cmd/order_api.swagger.json
.PHONY: order-proto

inventory-proto:
	mkdir -p docs
	
	protoc -I . \
		-I $(THIRD_PARTY) \
		--go_out=$(OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(OUT_DIR) --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(OUT_DIR) --grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=./docs --openapiv2_opt=logtostderr=true,allow_merge=true,merge_file_name=inventory_api \
		$(PROTO_INVENTORY_DIR)/inventory.proto

	cp docs/inventory_api.swagger.json ../services/inventory/cmd/inventory_api.swagger.json
.PHONY: inventory-proto