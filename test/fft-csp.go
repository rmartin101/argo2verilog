/*  Example program to compute an FFT using a CSP style of design 
 *
    (c) 2021, Richard P. Martin and contributers 
    
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


/* This code implements an FFT using a CSP style of design in Go
*  The program build a 2D butterfly. The nodes in the graph are Goroutines and the
*  edges are channels. Nodes are labeled in column-major order 
*    
input I(0)    ->  (0,0)--->(1,0)--->(2,0)--->(3,0)  -->output  X(0)
           \ /  
              ->  (0,1)--->(1,1)--->(2,1)--->(3,1)  -->output  X(1)
           \ /    
           ->  (0,2)--->(1,2)--->(2,2)--->(3,2)  -->output  X(2)
           / \ 
           ->  (0,3)--->(1,3)--->(2,3)--->(3,3)  -->output  X(3)       

References: 
  Fowler's notes:
  http://www.ws.binghamton.edu/Fowler/Fowler%20Personal%20Page/EE302_files/FFT%20Reading%20Material.pdf

*/
package main;
import ("fmt"); 
import ("math");
import ("runtime");
import ("time");
import ("github.com/dterei/gotsc");

const FFT_LOG uint32 = 3 ;  // log of the number of inputs/outputs
const FFT_LOG1 uint32 = (FFT_LOG+1)  // the nubmber of stages/columns of the FFT
const FFT_VSIZE uint32 = (1<<FFT_LOG) ;  // size of the input vector 
const FFT_NODES uint32 = (FFT_VSIZE * (FFT_LOG + 1) )  ; // number of nodes

const QUIT uint8 = 0xFF ;
const DEBUG_ON uint8 = 0xDE;
const DEBUG_OFF uint8 = 0x0D;

// make an array of channels 
// holds all the sizes of the FTT and channels of the FFT array
// each node has 2 inputs + 2 outputs, but outputs are shared as inputs
// except at the edges, which are the length of a vector
type FFTarray struct {
	input_channels [FFT_VSIZE]  chan complex128;   // input links
	shuffle_channels [FFT_VSIZE]  chan complex128;    // shuffle the data accourding to the bit-reversal 
	a_channels [FFT_LOG][FFT_VSIZE]   chan complex128;  // straigt across edges/links
	b_channels [FFT_LOG][FFT_VSIZE] chan complex128;  // b_channel edges/links
	output_channels [FFT_VSIZE]         chan complex128;  // output links
	cntl_channels  [FFT_LOG+1][FFT_VSIZE] chan uint8; // gorouting control for all nodes 
}

/* inp as a numbits number and bitreverses it. 
 * inp < 2^(numbits) for meaningful bit-reversal
 */ 
func bitrev(inp, numbits int) int {
	var i, rev int;

	i=0;
	rev = 0;
	for i=0; i < numbits; i++  {
		rev = (rev << 1) | (inp & 1);
		inp >>= 1;
	}
	return rev;
}

// for an FFT of size N, get the twiddle factor for a node at column c, row r
// recall the twiddle factor is the complex number on the unit circle
// higher levels (columns) break the circle into more parts
// see the reference 
func compute_twiddle_factor(col,row uint32) complex128 {
	var m,N uint32; 
	var inner float64;
	var retval complex128;
	
	N = (1<<(col+1));   // factors on the unit circle for a N-node FFT (2^col)-1
	m = row & (N -1)    // row % N , recall that x modulo y = (x & (y − 1))

	// recall e^-i*2*Pi*m/N = cos(2*Pi*m/N) - i*sin(2*Pi*m/N)
	inner = 2.0*math.Pi*float64(m)/float64(N) ; // fraction on the unit circle to move 
	retval = (complex(math.Cos(inner),-1.0*math.Sin(inner))) ; // definition of exponent above
	return retval;
}

// the input nodes copy one input to two output at the begining of the FFT
func input_node(row uint32, in chan complex128, out1 chan complex128, out2 chan complex128, control chan uint8 ) {
	var inputVal complex128;
	var msg uint8;
	var quit bool;
	var debug int;
	
	quit = false;
	// while quit == false 	
	for (quit == false) {
		inputVal = <- in;  // read a single input
		if (debug == 1) { 
			fmt.Printf("----input node (%d) got input %.3f \n",row,inputVal)
		}
		
		out1 <- inputVal;  // copy to the two outputs 
		out2 <- inputVal;

		// poll the control channel 
		if (len(control) > 0) {
			msg = <- control;
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
			};
		}; 

	};
};

// a compute node has a 2D address (columns,row), two inputs. two outputs, a twiddle factor,
// and a control channel.
// A compute node takes 2 inputs and sends the result to the two outputs

func compute_node(col uint32,row uint32, in1 chan complex128, in2 chan complex128, out1 chan complex128, out2 chan complex128, Wn complex128, control chan uint8 ) {
	var a,b,value complex128;
	var msg uint8;
	var quit bool;
	var debug int; 

	quit = false;
	debug = 0 ;
	
	// while quit == false 	
	for (quit == false) {
		
		a = <- in1;       // read the inputs
		b = <- in2;

	        value = a + Wn*b;  // this line is the main node computation

		if (debug == 1) {
			fmt.Printf("----compute node (%d:%d) inputs %.3f + %.3f * %.3f = %.3f \n",col,row,a,Wn,b,value)
		}
		
		out1 <- value;    // write the outputs
		if (col < (FFT_LOG-1)) { // last column only has 1 output channel 
			out2 <- value;
		}
		
		// poll the control channel 
		if (len(control) > 0) {
			msg = <- control;
			fmt.Printf("----compute node (%d:%d) control message %d \n",col,row,msg)
			switch msg {
			case QUIT:
				fmt.Printf("----compute node (%d:%d) ending \n",col,row,msg)
				return ; 				
			case DEBUG_ON:
				debug = 1;
			case DEBUG_OFF:
				debug = 0;
			default:
				fmt.Printf("----compute node (%d:%d) unknown message type %d \n",col,row,msg)				
			};
		}; 
	}; 
} ;

// set debugging, 1 is on, 0 is off 
func message_all(fft *FFTarray, message uint8) {
	var c, r int ;  // column and row 
	for c = 0; c <= int(FFT_LOG) ; c++ {   // we have a log(fftsize) columns
 		for r = 0; r < int(FFT_VSIZE) ; r++ {  // vector size rows 
			fft.cntl_channels[c][r] <- message;
		}
	}
	
}
func create_fft_array(fft *FFTarray) {
	const FFT_CHANNELS uint32 = (FFT_NODES *2) + (FFT_VSIZE*2)
	var r,c uint32; // the current column of row of the node to create
	var reversed int   // bit reversed of input index row 
	
	var twiddle complex128  // twiddle factor for a compute node 

	// these are use to compute the target row for the b_channel channels in the butterfly
	var cross_distance_out uint32 ; // number of rows from current row
	var cross_bit_value_out uint32;  // direction 0=decreasing (up) 1=down
	var cross_row_output uint32;  // the actual target row

	// indexes to the channels in the main channel arrays
	var channel1_out_id, channel2_out_id uint32
	
	// pointers to the channel in the channel arrays 
	var channel1_in, channel2_in, channel1_out, channel2_out chan complex128
	
	// make all the channels. The outer loop indexes the rows. The inner loop indexes the columns
	// we can set the inputs and outputs by row in the first outer loop 
	for r = 0; r < FFT_VSIZE ; r++ {
		fft.input_channels[r] =  make(chan complex128);
		fft.output_channels[r] =  make(chan complex128);
		for c = 0; c < FFT_LOG ; c++ {
			fft.a_channels[c][r]= make(chan complex128);
			fft.b_channels[c][r]= make(chan complex128);
			fft.cntl_channels[c][r] = make(chan uint8, 1);
		}
		fft.cntl_channels[FFT_LOG][r] = make(chan uint8, 1); // for the input splitter channels 
	}

	// this is the code that creates the butterfly using go routines and  channels 
	// recall an N-input FFT butterfly has log N levels
	// every level is a vector with a size. e.g. an 8 element FFT has 3 levels and each
	// vector length is 8; a 16 input FFT has 4 levels and the number of nodes
	// in each level/vector is 16
	// We use this terminology to organize a butterfly as a 2D array
	// each level (or stage) is a column and the vectors element number is the row
	// So a node is defined as a column-major array with nodes labeled as (c,r)
	// the 0th column is the input and the LogNth column is the output
	
	// each node is a go-routine connected by channels
	// loop for every level of the FFT, from outputs to inputs.
	// and create the nodes with the right channel interconnect

	// main loop to create compute nodes. For each column, for each row, create the node with
	// Channels are organized into the A channel set and B channels set. 
	// The A channels are the 'upper' input in the FFT diagram, and the B channels the 'lower' input
	// A channel's c,r value addressed the input of a a compute node.  So compute node A and B input channels 

	for c = 0; c < FFT_LOG ; c++ {   // we have a log(fftsize) columns 
		for r = 0; r < FFT_VSIZE ; r++ {  // vector size rows 

			// get the distance from the current row to the target row for the cross input channel 
			cross_distance_out = (1<<c)  // the output channel is one column to the right, so the distance is larger
			// the value of the bit in the row number determines if the output offset is up or down
			cross_bit_value_out = r & cross_distance_out
			
			// these are the output channel IDs for the A and B channels 
			if (cross_bit_value_out == 0) {
				cross_row_output = r + cross_distance_out
				channel1_out_id = r
				channel2_out_id = cross_row_output

				channel1_out = fft.a_channels[c][int(channel1_out_id)]
				channel2_out = fft.a_channels[c][int(channel2_out_id)]
				
			} else {
				cross_row_output = r - cross_distance_out
				channel1_out_id = cross_row_output 
				channel2_out_id = r

				channel1_out = fft.b_channels[c][int(channel1_out_id)]
				channel2_out = fft.b_channels[c][int(channel2_out_id)]
			}

			// logic to set the output nodes 
			if (c == 0) {  // the first layer needs input nodes
				reversed = bitrev(int(r),int(FFT_LOG));				
				go input_node(r,fft.input_channels[reversed],channel1_out,channel2_out,fft.cntl_channels[FFT_LOG][r]);
			} else { // compute layers
				twiddle = compute_twiddle_factor(c-1,r)
				channel1_in = fft.a_channels[c-1][int(r)]
				channel2_in = fft.b_channels[c-1][int(r)]
				go compute_node(c-1,r,channel1_in,channel2_in,channel1_out,channel2_out,twiddle,fft.cntl_channels[c][r])
			}
			// last layer needs output nodes 
			if (c == (FFT_LOG-1)) {
				twiddle = compute_twiddle_factor(c,r)
				channel1_in = fft.a_channels[c][int(r)]
				channel2_in = fft.b_channels[c][int(r)]				
				go compute_node(c,r,channel1_in,channel2_in,fft.output_channels[r],nil,twiddle,fft.cntl_channels[c][r])
			}
		}
	}; // end for compute nodes
}

func read_outputs(fft *FFTarray) {
	var value complex128;

	if (len(fft.output_channels) < 64 ) {
		for i, _ := range fft.output_channels {
			value = <- fft.output_channels[i] ;
			fmt.Printf("output: %d got val %.3f \n",i,value);
		}
	} 

}

func main() {
	var fft *FFTarray;
	var signal[FFT_VSIZE] float64 ;
	var i, j int;
	var t float64 ;
	//var cv complex128; 
	//var r,c int;
	
	runtime.GOMAXPROCS(8)
	
	fft = new(FFTarray); 
	create_fft_array(fft);

	// test the FFT by sending in some signals and make sure we get the
	// right values in the frequency domain

	// Make a square wave 
	for i =0; int(i) < len(signal) ; i++  {
		j = int(math.Floor(float64(i/2))) % 2
		if (j == 0) { 
			t = float64(1.0000);
		} else {
			t = float64(0.0000);
		}
		signal[i] = t;
	}


	
	// for i =0; i < len(signal) ; i++  {
	// 	fmt.Printf("signal[%d] is %.3f\n",i,signal[i]);		
	// 	fft.input_channels[i] <- complex(signal[i], 0.0) ; 
	// }
	// read_outputs(fft);

	message_all(fft,DEBUG_ON)
	for i =0; i < len(signal) ; i++  {
		fft.input_channels[i] <- complex(signal[i], 0.0) ; 
	}
	read_outputs(fft);
	
	tsc := gotsc.TSCOverhead()
	start := gotsc.BenchStart()
	for i =0; i < len(signal) ; i++  {
		fft.input_channels[i] <- complex(signal[i], 0.0) ; 
	}
	read_outputs(fft);
	end := gotsc.BenchEnd()
	time.Sleep(1 * time.Second)
	
	avg := (end - start - tsc)
	fmt.Println("TSC Overhead:", tsc)
	fmt.Println("Cycles:", avg)

}

