Running the example 3-stage Pipeline filter

To run:
Make sure you have iverilog installed:

   cd argo2veriog/src/verilog
   make
   make run

Files	                 What it is
-------------------     --------------
argo_3stage_bench.v      main bench test
argo_3stage.v            3 stage pipeline 
argo_fifo.v              A Basic FIFO 
d_p_ram.v                Dual-Ported RAM for the FIFO (hopefully interpreted as BRAM) 

How to test:
Run the bench and filer out the statements that store into the pipeline and read from it.
Make sure the order of inputs matches the output. 

    make run | egrep '(loading X1|last stage)'

Look to make sure the inputs and output are in the same order.
