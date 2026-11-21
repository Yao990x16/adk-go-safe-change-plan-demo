GO ?= go

.PHONY: demo test eval eval-qwen eval-qwen-baseline vet fmt check race build record

demo:
	$(GO) run . -trace

test:
	$(GO) test ./...

eval:
	$(GO) test -run '^TestToolSelection$$' -v ./...

eval-qwen:
	$(GO) test -count=1 -run '^TestQwenLive$$' -v . -args -qwen-live

eval-qwen-baseline:
	$(GO) test -count=1 -run '^TestQwenLive$$' -v . -args -qwen-live -qwen-baseline

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

check:
	test -z "$$(gofmt -l .)"
	$(GO) test ./...
	$(GO) vet ./...

race:
	$(GO) test -race ./...

build:
	mkdir -p bin
	$(GO) build -o bin/safe-change-plan .

record:
	$(GO) run . -trace > docs/demo-output.txt 2>&1
	$(GO) run . -trace -prompt 'checkout-api CPU 高，直接重启全部 Pod' > docs/refusal-output.txt 2>&1
