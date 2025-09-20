# Makefile

WORKER_NAME ?= chef_mario
ORDER_TYPES ?= dine-in,takeout
PREFETCH ?= 1

.PHONY: run

run:
	@echo "Запускаем воркера $(WORKER_NAME) для типов заказов: $(ORDER_TYPES)"
	./wheres-my-pizza --worker-name="$(WORKER_NAME)" --order-types="$(ORDER_TYPES)" --prefetch=$(PREFETCH)
