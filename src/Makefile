
ANTLR4=~/bin/antlr4 

all: argo2verilog.go genVerilog.go
	${ANTLR4} -Dlanguage=Go -o parser Argo.g4
	go build -gcflags 'all=-N -l' argo2verilog.go genVerilog.go 


install: argo2verilog 
	cp argo2verilog ../bin

.PHONY: clean run

clean:
	rm argo2verilog	

run:
	./argo2verilog -gv -i ../test/channel01.go





