FROM ysicing/god AS builder

WORKDIR /go/src

ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.mod

COPY go.sum go.sum

RUN go mod download

COPY . .

RUN make build

FROM ysicing/debian

COPY --from=builder /go/src/dist/kubetls /usr/bin/kubetls

RUN chmod +x /usr/bin/kubetls

CMD /usr/bin/kubetls
