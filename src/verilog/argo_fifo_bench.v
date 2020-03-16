/* fifo testbench */

module argo_fifo_bench();


   parameter MAX_CYCLES = 100;
   parameter PIPE_1_WIDTH = 32;
   parameter PIPE_1_ADDR_WIDTH = 4;
    
   parameter PIPE_2_WIDTH = 32;
   parameter PIPE_2_ADDR_WIDTH = 3;
   
   /* variables */
   reg [31: 0] Y1;  // write into FIFO 1
   reg [31: 0] X1;  // Read from FIFO 1
   reg [31: 0] Z1;  // write to FIFO 2

   /* control regs */
   reg clk ; // clock 
   reg rst ; // reset 
   reg [63:0] cycle_count ;  // cycle counter for performance and debugging 
   reg c_bit_00000_start ;  // initial state control
   reg c_bit_00001;  // the control bits write into pipe 1
   reg c_bit_00002;
   reg c_bit_00003;
   reg c_bit_00004;
   reg c_bit_00005;
   reg c_bit_00006;   
   reg c_bit_00007;
   reg c_bit_00008;   

   // move data from pipe 1 to pipe 2
   reg c_bit_00101;  // these control read from pipe 1
   reg c_bit_00102;  // and write to pipe 2
   reg c_bit_00103;
   reg c_bit_00104;
   reg c_bit_00105;
   reg c_bit_00106;   
   reg c_bit_00107;
   reg c_bit_00108;   

   // Pipe/channel 1 registers    
   reg [PIPE_1_WIDTH-1:0 ] pipe_1_write_data;
   wire [PIPE_1_WIDTH-1:0 ] pipe_1_read_data;
   reg pipe_1_rd_en_reg;
   reg pipe_1_wr_en_reg;   
   wire pipe_1_full;
   wire pipe_1_empty;

   reg [PIPE_2_WIDTH-1:0 ] pipe_1_write_data;
   wire [PIPE_2_WIDTH-1:0 ] pipe_1_read_data;
   reg pipe_2_rd_en_reg;
   reg pipe_2_wr_en_reg;   
   wire pipe_2_full;
   wire pipe_2_empty;

/* channels */   
argo_fifo #(.ADDR_WIDTH(PIPE_1_ADDR_WIDTH),.DATA_WIDTH(PIPE_1_WIDTH),.DEPTH(1<<PIPE_1_ADDR_WIDTH)) PIPE_1 (
    .clk(clk),
    .rst(rst),		 
    .rd_en(pipe_1_rd_en_reg),
    .rd_data(pipe_1_read_data),
    .wr_en(pipe_1_wr_en_reg),
    .wr_data(pipe_1_write_data),
    .full(pipe_1_full),
    .empty(pipe_1_empty)
);
 

argo_fifo #(.ADDR_WIDTH(PIPE_2_ADDR_WIDTH),.DATA_WIDTH(PIPE_2_WIDTH),.DEPTH(1<<PIPE_2_ADDR_WIDTH)) PIPE_2 (
    .clk(clk),
    .rst(rst),		 
    .rd_en(pipe_2_rd_en_reg),
    .rd_data(pipe_2_read_data),
    .wr_en(pipe_2_wr_en_reg),
    .wr_data(pipe_2_write_data),
    .full(pipe_2_full),
    .empty(pipe_2_empty)
);
		  
initial begin 
   clk =0;
   Y1=0;
   cycle_count =0 ;

   pipe_1_rd_en_reg =0;
   pipe_1_wr_en_reg =0;
   pipe_1_write_data =0;
   
   c_bit_00001 =0 ;
   c_bit_00002 =0 ;
   c_bit_00003 =0 ;
   c_bit_00004 =0 ;
   c_bit_00005 =0 ;
   c_bit_00006 =0 ;   
   c_bit_00007 =0 ;
   c_bit_00008 =0 ;

   c_bit_00101 =0 ;
   c_bit_00102 =0 ;
   c_bit_00103 =0 ;
   c_bit_00104 =0 ;
   c_bit_00105 =0 ;
   c_bit_00106 =0 ;   
   c_bit_00107 =0 ;
   c_bit_00108 =0 ;
   
   rst = 1;
   c_bit_00000_start = 1 ;
   #1;
   clk = 1;
   #10;
   rst = 0;
   #10;   
end // initial 

// ************ Data flow section ********************* */
always @(posedge clk) begin // Data flow for Y1 
   if (c_bit_00001 == 1) begin
      Y1 <= Y1 + 1;
      $display("incrementing Y1 val %d at cycle %d",Y1,cycle_count);
   end else begin
      Y1 <= Y1 ;      
      end 
end

/**** channel 1 writer data flow section *********/
always @(posedge clk) begin // control for line c_bit_00001;
   if (c_bit_00003 == 1) begin
      pipe_1_write_data <= Y1 ;
      pipe_1_wr_en_reg <= 1 ;
      $display("storing Y1 into  pipe 1 val: %d cycle %d",Y1,cycle_count);
   end else begin 
      pipe_1_write_data <= 0 ;
      pipe_1_wr_en_reg <= 0 ;
   end
end


/**** channel 1 reader data  flow section ****/
always @(posedge clk) begin // data flow for read enable on FIFO 1 
   if ((c_bit_00105 == 1) && (!(pipe_1_empty )))begin
      pipe_1_rd_en_reg <= 1 ;
      $display("enabling read on pipe 1 cycle %d",cycle_count);
      end else begin 
	 pipe_1_rd_en_reg <= 0 ;
   end
end

always @(posedge clk) begin // data flow for reads of the filo 
   if (c_bit_00106 == 1) begin
      X1 <= pipe_1_read_data;
      $display("enabling read on pipe 1 %d cycle %d",X1,cycle_count);
   end else begin 
      X1 <= X1;
   end
end
   
// ************ control flow section for PIPE 1 Writer ********************* */
always @(posedge clk) begin // control for line c_bit_00001;
 	 if ((c_bit_00000_start == 1) || (c_bit_00006 == 1))  begin
	    c_bit_00000_start <= 0;
 	    c_bit_00001 <= 1 ;
	    $display("at control line 1 cycle count %d ",cycle_count);
 	 end 
 	 else  begin
	    c_bit_00000_start <= 0;
 	    c_bit_00001 <= 0 ;  
 	 end
end // end @ posedge clk

   
always @(posedge clk) begin // control for line c_bit_00002;
   if ( (c_bit_00001 == 1) )begin
      c_bit_00002 <= 1 ;
      $display("at control line 2 cycle count %d",cycle_count);
   end 
   else if ( (c_bit_00002 == 1 ) && ( pipe_1_full ) ) begin 
      c_bit_00002 <= 1;
      $display("waiting for pipe 1 to clear cycle %d",cycle_count);      
   end else begin
      c_bit_00002 <= 0;  	
   end 
   
end // end @ posedge clk
   
always @(posedge clk) begin // control for line c_bit_00003;
 	 if ( (c_bit_00002 == 1) && (!(pipe_1_full))) begin 
 	    c_bit_00003 <= 1 ;
	    $display("at control line 3 cycle count %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00003 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00003;
 	 if ( (c_bit_00003 == 1)) begin 
 	    c_bit_00004 <= 1 ;
	    $display("at control line 4 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00004 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00005;
 	 if ( (c_bit_00004 == 1)) begin 
 	    c_bit_00005 <= 1 ;
	    $display("at control line 5 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00005 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00006;
 	 if ( (c_bit_00005 == 1)) begin 
 	    c_bit_00006 <= 1 ;
	    $display("at control line 6 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00006 <= 0 ;  
 	 end
end // end @ posedge clk

/*  *********** control flow section for PIPE 2 Reader and Writer ********************* */   

always @(posedge clk) begin // control for line c_bit_00101;
 	 if ((c_bit_00000_start == 1) || (c_bit_00108 == 1))  begin
 	    c_bit_00101 <= 1 ;
	    $display("at control line 101 cycle count %d ",cycle_count);
 	 end 
 	 else  begin
 	    c_bit_00101 <= 0 ;  
 	 end
end // end @ posedge clk

   
always @(posedge clk) begin // control for line c_bit_0102;
   if ( (c_bit_00101 == 1) )begin
      c_bit_00102 <= 1 ;
      $display("at control line 102 cycle count %d",cycle_count);
   end 
   else if ( (c_bit_00102 == 1 ) && ( pipe_1_empty ) ) begin 
      c_bit_00102 <= 1;
      $display("waiting for pipe 1 to have data cycle %d",cycle_count);      
   end else begin
      c_bit_00102 <= 0;  	
   end 
   
end // end @ posedge clk
   
always @(posedge clk) begin // control for line c_bit_00103;
 	 if ( (c_bit_00102 == 1) && (!(pipe_1_empty))) begin 
 	    c_bit_00103 <= 1 ;
	    $display("at control line 3 cycle count %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00103 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00104;
 	 if ( (c_bit_00103 == 1)) begin 
 	    c_bit_00104 <= 1 ;
	    $display("at control line 4 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00104 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00105;
 	 if ( (c_bit_00104 == 1)) begin 
 	    c_bit_00105 <= 1 ;
	    $display("at control line 5 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00105 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00106;
 	 if ( (c_bit_00105 == 1)) begin 
 	    c_bit_00106 <= 1 ;
	    $display("at control line 6 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00106 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00107;
 	 if ( (c_bit_00106 == 1)) begin 
 	    c_bit_00107 <= 1 ;
	    $display("at control line 6 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00107 <= 0 ;  
 	 end
end // end @ posedge clk
   
always @(posedge clk) begin // control for line c_bit_00108;
 	 if ( (c_bit_00107 == 1)) begin 
 	    c_bit_00108 <= 1 ;
	    $display("at control line 6 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00108 <= 0 ;  
 	 end
end // end @ posedge clk//    
   
/* *********** cycle counter ***********************/  
always @(posedge clk) begin
   if (cycle_count > MAX_CYCLES) begin
      $display("reached maximum cycle count of %d ",MAX_CYCLES);
      $finish;
   end
   else begin
      cycle_count <= cycle_count + 1 ;
   end
 end 

/* clock control for the test bench */   
always begin 
  #1 clk = !clk ; 
end 

   
endmodule // argo_fifo_bench

