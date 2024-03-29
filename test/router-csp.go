/*  Example program of a randomized butterfly router design for IP packets 
 *
    (c) 2022 Richard P. Martin and contributers 
    
    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License Version 3 for more details.t

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>
*/


/* This code implements randomized butterfly routing using a CSP style of design in Go.
*  The architecture is to build back-to-back, 2D butterflies. The nodes in the butterfly graph 
*  are Goroutines and the edges are channels.
*  
* Nodes are labeled in column-major order as below, with the bufferfly going from left to right: 
input I(0)    ->  (0,0)--->(1,0)--->(2,0)--->(3,0)  -->output  X(0)
           \ /  
              ->  (0,1)--->(1,1)--->(2,1)--->(3,1)  -->output  X(1)
           \ /    
           ->  (0,2)--->(1,2)--->(2,2)--->(3,2)  -->output  X(2)
           / \ 
           ->  (0,3)--->(1,3)--->(2,3)--->(3,3)  -->output  X(3)       

* The first column are input nodes. Each input node also has a linear-feedback shift register 
* node, which is used to create random numbers. 
* The interior nodes are routing nodes. The last column are output nodes. 

* When a packet arrives, it is assumed to be labed with the output port. 
* The input node then selects a random path to the end of the first butterfly.
* The input node then computes the path from the interior node to the correct output node. 

References: 
Simple Algorithms for Routing on Butterfly Networks with Bounded Queues (Extended Abstract)
Bruce M. Maggs and Ramesh K. Sitaraman
"https://users.cs.duke.edu/~bmm/assets/pubs/MaggsS92-STOC.pdf" 

*/
package main;
import ("fmt"); 
import ("runtime");
import ("flag");
import ("time");
import ("os") ;
import ("github.com/dterei/gotsc");

// make an array of channels 
// holds all the sizes of the FTT and channels of the FFT array
// each node has 2 inputs + 2 outputs, but outputs are shared as inputs
// except at the edges, which are the length of a vector


// an IP version 4 header. Tries to 
type IPv4hdr struct {
	Version_Len  uint8       // protocol version, header length (4 bits each) 
	TOS          uint8       // type-of-service
	TotalLen     uint16      // packet total length
	ID           uint16      // packet identification number 
	Flags_Offset    uint16       // flags (3 bits) and the fragement offset (13 bits) 
	TTL      uint8       // time-to-live
	Protocol uint8       // next protocol
	Checksum uint16      // checksum
	Src      uint32      // source address
	Dst      uint32      // destination address
	// Options  uint32    Options field 
} ;

type RouterPkt struct {
	dest_port uint16;    // destination port 
	path uint32;  // bit-mapped path through the router. 0 is a straight link and 1 is a cross link.
	header IPv4hdr;    // an IP version 4 header 
} 

// constants that define the size of router. The router log is the base-2 log of the number of inputs
const ROUTER_LOG uint32 = 2 ;  // log (base 2) of the number of inputs/outputs
const ROUTER_LOG1 uint32 = (ROUTER_LOG+1)  // the nubmber of stages/columns of the first butterfly with the input column
const ROUTER_ISIZE uint32 = (1<<ROUTER_LOG) ;  // number of inputs to the router
const ROUTER_DEPTH uint32 = ((ROUTER_LOG1) + (ROUTER_LOG)) // depth in number of nodes, includes 2nd butterfly
const ROUTER_INPUT_NODES = ( ROUTER_ISIZE )
const ROUTER_OUTPUT_NODES = ( ROUTER_ISIZE ) 
const ROUTER_NODES uint32 = ( (ROUTER_ISIZE * (ROUTER_LOG + 1)) + ((ROUTER_ISIZE * ROUTER_LOG)))  ; // total number of nodes
const ROUTER_RT_NODES = (ROUTER_NODES - (ROUTER_INPUT_NODES + ROUTER_OUTPUT_NODES)) // internal routing nodes 

const QUIT uint8 = 0xFF ;
const DEBUG_ON uint8 = 0xDE;
const DEBUG_OFF uint8 = 0x0D;

type RouterState struct {
	input_channels [ROUTER_ISIZE]  chan RouterPkt;                  // input channels
	random_num_channels [ROUTER_ISIZE] chan uint8;                   // for random numbers for the inputs
	output_channels [ROUTER_ISIZE]         chan RouterPkt;          // output channels
	straight_channels [ROUTER_DEPTH][ROUTER_ISIZE]   chan RouterPkt;  // straigt across edges/links
	cross_channels [ROUTER_DEPTH][ROUTER_ISIZE] chan RouterPkt;       // cross channel edges/links
	cntl_channels[ROUTER_DEPTH+1][ROUTER_ISIZE] chan uint8;          // gorouting control for all nodes and the lfsr in the extra column.
}

// a linear feedback shift register used for generating a psuedo-random sequence of 0s or 1s .
// the stream of 0/1s is put on an output channel.
func lfsr3(row uint32, seed uint16, sequence chan uint8, control chan uint8) {
	var lfsr uint16;  // the linear feedback shift register
	var value uint16; 
	var stop bool;
	var msg uint8;    // control message 
	var debug int;

	debug =1 ;
	if (debug == 1) {fmt.Printf("%d,0 lfsr node %x started \n",row, sequence) };
	
	stop = false ;
	lfsr = seed;
	for stop != true {
		select {
		case msg = <- control:
			if (debug ==1) { fmt.Printf("----lfsr3 %d (%d) control message %d \n",row,seed,msg) }; 
			switch msg {
			case QUIT:
				if (debug ==1) {fmt.Printf("----lfsr3 node %d (%d) ending \n",row,seed,msg)}; 
				stop = true;
			case DEBUG_ON:
				debug = 1;
			case DEBUG_OFF:
				debug = 0;
			default:
				fmt.Printf("----lfsr node %d (%d) unknown message type %d \n",row,seed,msg)
			} ;
		default: 
			//shifts
			lfsr ^= lfsr >> 7 ; // shift 7 bits right
			lfsr ^= lfsr << 9 ;  // shift 9 bits left
			lfsr ^= lfsr >> 13 ; // shift 13 bits right

			fmt.Printf("lfsr is x%x \n",lfsr);
			
			//adds last bit to the output channel 
			value = lfsr & 1 ;
			if (value == 0) {
				sequence <- 0;
			} else { 
				sequence <- 1; 
			}
			runtime.Gosched();  // throw back control to the scheduler 
		} // end select 
	} // end for 
}

// the input nodes copy one input to two output at the begining of the FFT
func input_node(col uint32, row uint32, rand_input chan uint8, in chan RouterPkt, straight chan RouterPkt, cross chan RouterPkt, control chan uint8 ) {
	var inputPkt RouterPkt;
	var msg uint8;
	var quit bool;
	var debug int;
	var set_bit_position uint16 ;
	var rand_bit uint16; 
	var rand_path uint16;
	var dest_path uint16;
	var dest_port uint16; 
	var i int;
	
	// starting position of the linear feedback shift register 
	const my_column = 0;
	quit = false;
	debug =1 ;
	if (debug == 1) {fmt.Printf("%d,%d input node started rand:%x input:%x straight:%x cross:%x\n",col,row,rand_input,in,straight,cross) };
	// while quit == false 	
	for (quit == false) {
		// poll the control channel
		select {
		case inputPkt = <- in:  // read a single input packet

			// create a path to a central node in the butterfly using random bits 
			rand_path = 0;
							
			fmt.Printf("test lfsr: ");
			for i =0; i< 48; i++ {
				rand_bit = uint16(<- rand_input);  // get the next bit from the LFSR channel
				fmt.Printf(":%x:",rand_bit);
			}
			fmt.Printf("\n");
			for i =0; i< int(ROUTER_LOG); i++ {
				rand_bit = uint16(<- rand_input);  // get the next bit from the LFSR channel 
				rand_path = ((rand_bit & 0x1 ) << i) | rand_path;   // add it to the path 
			}
			// compute the path from the central node to the output port 
			dest_port = inputPkt.dest_port; 
			dest_path = 0;

			// we do a low-order bit by bit comparison between the path and the
			// destination port. If the bits do not match, take the cross link/channel 
			// if the bits differ, take the cross link. 
			for i =0; i< int(ROUTER_LOG); i++ {
				set_bit_position = (1<<i) ;
				if (dest_port & set_bit_position) != (rand_path & set_bit_position) {
					dest_path = dest_path | set_bit_position;
				}
			}


			
			// the full path is the random path followed by the final destination 
			inputPkt.path = uint32(rand_path)<<16 | uint32(dest_path); 			

			fmt.Printf("----dest %d rand x%x path x%x \n",dest_port,rand_path,dest_path);
			
			
			if (debug == 1) { 
				fmt.Printf("----input node (%d) got input %s rand_path %x dest_path %x\n",row,inputPkt,rand_path,dest_path);
			}
			
			// check the routing bit if the packet goes on the straight or cross channel
			if ((dest_path & 1) == ( uint16(col) & 1 )) { 
				cross <- inputPkt;  // copy to the two outputs
				if (debug == 1) { fmt.Printf("----input node (%d) sent cross \n",row); }
			} else { 
				straight <- inputPkt;
				if (debug == 1) { fmt.Printf("----input node (%d) sent straight \n",row); }
			}
			
		case msg = <- control:
			fmt.Printf("----input node (%d) control message %d \n",row,msg)
			switch msg {
			case QUIT:
				fmt.Printf("----input node (%d) ending \n",row,msg)
				return ; 				
			case DEBUG_ON:
				debug = 1;
			case DEBUG_OFF:
				debug = 0;
			default:
				fmt.Printf("----input node (%d) unknown message type %d \n",row,msg)
			} ;
		};
	};
};

// a compute node has a 2D address (columns,row), two inputs. two outputs,
// and a control channel.
// A compute node takes 2 inputs and sends the result to the two outputs

func routing_node(col uint32,row uint32, straight_in chan RouterPkt, cross_in chan RouterPkt, straight_out chan RouterPkt, cross_out chan RouterPkt, control chan uint8 ) {
	var inputPkt RouterPkt;
	var msg uint8;
	var quit bool;
	var debug int; 
	var routing_bit uint32;
	
	quit = false;
	debug = 1 ;
	routing_bit = (1 << col) ;

	if (debug == 1) {fmt.Printf("%d,%d routing node started straight_in:%x cross_in:%x straight_out:%x cross_out:%x \n",col,row,straight_in,cross_in,straight_out, cross_out) }; 
		
	// while quit == false 	
	for (quit == false) {
		select {
		case inputPkt = <- straight_in:    // read and input packet 
			if (debug == 1) { fmt.Printf("---routing node (%d_%d) in-straight packet %x \n",col,row,inputPkt); } ;

			// if the routing bit matches the nodes position in the bit-mask, go straight
			// else go on the cross link. 
			if ( (inputPkt.path & routing_bit ) == (col & routing_bit) ) {
				straight_out <- inputPkt;
			} else {
				cross_out <- inputPkt;
			}
			
		case inputPkt = <- cross_in:  // read an input packet 
			if (debug == 1) { fmt.Printf("---routing node (%d_%d) in-cross packet %x \n",col,row,inputPkt); } ;
			
			if ( (inputPkt.path & routing_bit ) == (col & routing_bit) ) {
				straight_out <- inputPkt;
			} else {
				cross_out <- inputPkt;
			}

		case msg = <- control:
			fmt.Printf("----routing node (%d:%d) control message %d \n",col,row,msg)
		switch msg {
		case QUIT:
			fmt.Printf("----routing node (%d:%d) ending \n",col,row,msg)
			quit = true; 
			return ; 				
		case DEBUG_ON:
			debug = 1;
		case DEBUG_OFF:
			debug = 0;
		default:
			fmt.Printf("----routing node (%d:%d) unknown message type %d \n",col,row,msg);
		}; // end switch 
			
		}; // end select
	}; 
} ;

// an output takes 2 inputs and multiplexes them onto one output.
func output_node(col uint32,row uint32, straight chan RouterPkt, cross chan RouterPkt, output chan RouterPkt, control chan uint8 ) {
	var inputPkt RouterPkt;
	var msg uint8;
	var quit bool;
	var debug int; 

	quit = false;
	debug =1 ;
	if (debug == 1) {fmt.Printf("%d,%d output node started straight:%x cross:%x output:%x\n",col,row, straight, cross, output) };

	for (quit == false) {
		select {
		case inputPkt = <- straight:    // read and input packet 
			if (debug == 1) { fmt.Printf("---output node (%d_%d) in-straight packet %x \n",col,row,inputPkt); } ;
			output <- inputPkt;    // write the outputs

		case inputPkt = <- cross:  // read an input packet 
			if (debug == 1) { fmt.Printf("---output node (%d_%d) in-cross packet %x \n",col,row,inputPkt); } ;
			output <- inputPkt;    // write the outputs
		
		case msg = <- control:
			fmt.Printf("----output node (%d:%d) control message %d \n",col,row,msg)
		switch msg {
		case QUIT:
			fmt.Printf("----output node (%d:%d) ending \n",col,row,msg)
			quit = true; 
			return ; 				
		case DEBUG_ON:
			debug = 1;
		case DEBUG_OFF:
			debug = 0;
		default:
			fmt.Printf("----output node (%d:%d) unknown message type %d \n",col,row,msg);
		}; // end switch 
			
		}; // end select
	}; // end for 
}; // end function 

// set debugging and other messages, 1 is on, 0 is off 
func message_all(router *RouterState, message uint8) {
	var c, r int ;  // column and row
	for r = 0; r < int(ROUTER_ISIZE) ; r++ {  // vector size rows
		for c = 0; c < int(ROUTER_DEPTH)+1 ; c++ {   // recall the lfsr nodes are the extra column
			fmt.Printf("sending message 0x%x to node at (%d:%d) \n",message,c,r)
			router.cntl_channels[c][r] <- message;
		} ;
	}; 
};

func create_router_state(router *RouterState) {
	var last_column,mid_column,column uint32;   // we have 2 back-to-back butterflys, but they share a column
	var second_bfly_col uint32;  // for the 2nd attached butterfly, the equalivant col in the 1st bfly
	var r,c uint32; // the current column of row of the node to create

	// these are use to compute the target row for the b_channel channels in the butterfly
	var cross_distance_out uint32 ; // number of rows from current row
	var cross_bit_value_out uint32;  // direction 0=decreasing (up) 1=down
	var cross_row_output uint32;  // the actual target row

	// indexes to the channels in the main channel arrays
	var channel1_out_id, channel2_out_id uint32 ; 
	
	// pointers to the channel in the channel arrays 
	var channel1_in, channel2_in, channel1_out, channel2_out chan RouterPkt; 

	last_column = ROUTER_DEPTH;
	mid_column = ROUTER_LOG ;

	// make all the channels. The outer loop indexes the rows. The inner loop indexes the columns
	// we can set the inputs and outputs by row in the first outer loop 
	for r = 0; r < ROUTER_ISIZE ; r++ {
		router.input_channels[r] =  make(chan RouterPkt);
		router.random_num_channels[r] =  make(chan uint8);
		router.output_channels[r] =  make(chan RouterPkt);
		router.cntl_channels[ROUTER_DEPTH][r] = make(chan uint8);
		
		for c = 0; c < last_column; c++ {
			router.straight_channels[c][r]= make(chan RouterPkt);
			router.cross_channels[c][r]= make(chan RouterPkt);
			router.cntl_channels[c][r] = make(chan uint8);
		} ;
	} ;

	// This nested loop creates the butterfly using go routines and channels.
	// Recall an N-input ROUTER butterfly has log N levels
	// every level is a vector with a size. e.g. an 8 element ROUTER has 3 levels and each
	// vector length is 8; a 16 input ROUTER has 4 levels and the number of nodes
	// in each level/vector is 16.
	// 
	// We use a 2D terminology to organize a butterfly as a 2D array.
	// Each level (or stage) is a column and the vectors element number is the row
	// So a node is defined as a column-major array with nodes labeled as column, row (c,r).
	// the 0th column is the input and the LogNth column is the output.
	// Thus there is an outer loop for the columns and inner loop for the rows. 
	// Data in the butterfly flows from left to right.
	
	// For a network router, we have 2 back-to-back butterflys that share a column to form a Benes network.
	// The 2nd butterfly is reversed from the first. The variable mid_column defines the shared column.
	// the cross distance is subtracted from the final column as it has to work
	// in reverse from the forward direction. 
	
	// A node is realized as a go-routine connected by channels. 
	// The loops run for every level of the router, from outputs to inputs.
	// and create the nodes with the right channel interconnect

	// Channels are organized into the straight channel set and cross channels set. 
	// The straight channels are the 'upper' input in the ROUTER diagram, and the cross channels the 'lower' input
	// A channel's c,r value addressed the input of a router node.
	
	for column = 0; column < last_column ; column++ {   // we have depth (depth = log+1 + log) column
		c = column; 
		for r = 0; r < ROUTER_ISIZE ; r++ {  // input size rows 
			second_bfly_col =  last_column +1 ;
			// get the distance from the current row to the target row for the cross input channel			
			if (c < mid_column) {
				cross_distance_out = (1<< column);  // the output channel is one column to the right, so the distance is larger
			} else {
				second_bfly_col = ((last_column-1) - c)-1; 
				cross_distance_out = (1<< second_bfly_col);  // the output channel is one column to the right, so the distance is larger
			}
			// the value of the bit in the row number determines if the output offset is up or down
			cross_bit_value_out = r & cross_distance_out; 

			// these are the output channel IDs for the straight and cross channels 
			if (cross_bit_value_out == 0) {
				cross_row_output = r + cross_distance_out ;
			} else {
				cross_row_output = r - cross_distance_out ; 
			}
			channel1_out_id = r  ; 
			channel2_out_id = cross_row_output ; 

			fmt.Printf("Loop %d_%d starting straight:%d cross:%d dist:%d \n", column,r,channel1_out_id,channel2_out_id,cross_distance_out);
			channel1_out = router.straight_channels[c][int(channel1_out_id)] ;
			channel2_out = router.cross_channels[c][int(channel2_out_id)] ;
				

			// Check which node types to create in this loop iteration. Each node in the butterfly is a goroutine. 
			// There are inputs, outputs, and routing nodes. Each node type has different channel connections. 
			if (column == 0) {  // first layer is input nodes 
				fmt.Printf("%d_%d starting lfsr \n",ROUTER_DEPTH,r);
				go lfsr3(r,uint16(r+(1<<r)|101),router.random_num_channels[r], router.cntl_channels[ROUTER_DEPTH][int(r)]);
				fmt.Printf("%d_%d starting input node \n",0,r);
				go input_node(c,r,router.random_num_channels[r],router.input_channels[r],channel1_out,channel2_out,router.cntl_channels[0][r]) ;

			} else if (column == (last_column-1))  {
				// last layer needs output nodes with 2 inputs, 1 output 
				channel1_in = router.straight_channels[c-1][int(r)]
				channel2_in = router.cross_channels[c-1][int(r)]
				fmt.Printf("%d_%d starting output node \n",c,r);
				go output_node(c,r,channel1_in,channel2_in,router.output_channels[r],router.cntl_channels[c+1][r])
			} else { 
				// interior nodes are routers with 2 inputs, 2 outputs 
				channel1_in = router.straight_channels[c-1][int(r)]
				channel2_in = router.cross_channels[c-1][int(r)]
				fmt.Printf("%d_%d starting router node \n",c,r);
				go routing_node(c,r,channel1_in,channel2_in,channel1_out,channel2_out,router.cntl_channels[c][r]) 
			}
		}; // end for rows 
	}; // end for columns
}

// write a bunch of packets to the inputs 
func write_inputs(router *RouterState,iterations int,printIt bool) {
	var i int ; 
	var inputPkt RouterPkt;
	for i =0; i< iterations; i++ {
		for j, _ := range router.output_channels {
			inputPkt.dest_port = uint16((uint32(j) % ROUTER_ISIZE)); 
			router.input_channels[j] <- inputPkt ;
			if printIt {
				fmt.Printf("write_input: sent %d val %x \n",j,inputPkt);
			} ;
		}; 
	} ;

	if printIt {
		fmt.Printf("write_input: done with input \n");
	}

}; 
// read packets from the outputs 
func read_outputs(router *RouterState,iterations int,printIt bool,done chan bool) {
	var i int ; 
	var outputPkt RouterPkt;

	for i =0; i< iterations; i++ {
		for j, _ := range router.output_channels {
			fmt.Printf("read_outputs: about to read channel %d \n " ,j);
			outputPkt = <- router.output_channels[j] ;
			if printIt {
				fmt.Printf("read_outputs: %d got val %s\n",j,outputPkt);
			} ;
		} ;
	}  ;
	done <- true;
}

func main() {
	var router *RouterState;       // holds the array of channels 
	var goProcsFlag_p *int ; // flag pointer to set number of procs 
	var debugFlag_p *bool ;  // debug flag pointer   
	var iterations_p *int;   // number of iterations to warm up the cache 
	
	var procsFlag int ;     // nunber of goprocs 
	var debugFlag bool ;    
	var iterations int ;
	var lapsed_nano int64 ;
	var doneChan chan bool ;
	var done bool ; 
	
	debugFlag_p = flag.Bool("d",false,"enable debugging") ; 
	goProcsFlag_p = flag.Int("p",1,"set GOMAXPROCS") ; 
	iterations_p = flag.Int("i",1,"set iterations") ; 
	flag.Parse() ; 

	procsFlag = *goProcsFlag_p;
	debugFlag = *debugFlag_p;
	iterations = *iterations_p;
	
	// get the maximum number of go processes to use from the arguments
	runtime.GOMAXPROCS(procsFlag) ; 
	
	router = new(RouterState); 
	create_router_state(router);
	doneChan = make(chan bool,1) ;

	// send a debugging message to all the channels 
	if (debugFlag == true) { 
		message_all(router,DEBUG_ON) ;
	} ;

	time.Sleep(3);
	//os.Exit(1);
	//done = <- doneChan ;
	
	// warm up
	write_inputs(router,1,true);
	read_outputs(router, 1,true,doneChan);
	done = <- doneChan ; 
	
	start_time := time.Now().UnixNano() ; 
	tsc := gotsc.TSCOverhead()  ; 
	start := gotsc.BenchStart() ; 
	go read_outputs(router,iterations,false,doneChan)  ; 
	go write_inputs(router,iterations,false);
	done = <- doneChan ; 
	
	end := gotsc.BenchEnd()  ; 
	end_time := time.Now().UnixNano() ; 

	lapsed_nano = int64(end_time) - int64(start_time)  ; 
	avg := (end - start - tsc) ; 
	//fmt.Println("TSC Overhead:", tsc)
	//fmt.Println("Cycles:", avg)
	fmt.Printf("%d,%d,%d,%d,%d,%t\n",ROUTER_ISIZE,iterations,procsFlag,avg,lapsed_nano,done);

	os.Exit(1);
} ;



















