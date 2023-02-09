NAME         ?= check_database

# build specific flags
DESTDIR      ?= .
prefix       ?= /usr/local
exec_prefix  ?= ${prefix}
bindir       ?= ${exec_prefix}/bin
sysconfdir   ?= ${prefix}/etc/${NAME}
WRKDIR       ?= build
GOBIN        ?= go

# set GOOS to linux by default
GOOS         ?= linux
BUILDID      = 0x`head -c20 /dev/urandom | od -An -tx | tr -d ' \n'`
LDFLAGS      += -B ${BUILDID}
BUILD_DATE   ?= `date +%FT%T%z`
LDFLAGS      += -X main.BUILD_DATE=${BUILD_DATE}
GOFLAGS      ?= -mod=vendor

clean:
	-rm -r ${WRKDIR}

build: clean
	mkdir -p ${WRKDIR}
	GOOS=${GOOS} GOFLAGS=${GOFLAGS} CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o ${WRKDIR}/${NAME}

install: build
	install -d -m 0755 ${DESTDIR}${bindir}
	install -d -m 0755 ${DESTDIR}${sysconfdir}
	install -m 0755 ${WRKDIR}/${NAME} ${DESTDIR}${bindir}
	install -m 0700 check_database.conf.example ${DESTDIR}${sysconfdir}

package: DESTDIR = ${NAME}-${VERSION}
package: install
	tar -czf ${NAME}-${VERSION}.tar.gz ${DESTDIR}
	rm -R ${DESTDIR}
