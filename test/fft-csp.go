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
import ("time");
import ("os");

const FFT_LOG uint32 = 3 ;  // log of the number of inputs/outputs
const FFT_LOG1 uint32 = (FFT_LOG+1)  // the nubmber of stages/columns of the FFT
const FFT_VSIZE uint32 = (1<<FFT_LOG) ;  // size of the input vector 
const FFT_NODES uint32 = (FFT_VSIZE * (FFT_LOG + 1) )  ; // number of nodes

// holds all the sizes of the FTT and channels of the FFT array 
type FFTarray struct {
	shuffle_channels [FFT_VSIZE]  chan complex128;    // shuffle the data accourding to the bit-reversal 
	input_channels [FFT_VSIZE]  chan complex128;   // input links
	a_channels [FFT_LOG][FFT_VSIZE]   chan complex128;  // straigt across edges/links
	b_channels [FFT_LOG][FFT_VSIZE] chan complex128;  // b_channel edges/links
	output_channels [FFT_VSIZE]         chan complex128;  // output links
	cntl_channels  [FFT_LOG+1][FFT_VSIZE] chan uint8; // gorouting control 
}

func shuffle_node(row uint32, in chan complex128, out chan complex128, control chan uint8 ) {
	var inputVal complex128;
	var msg uint8;
	var quit bool;

	// while quit == false 	
	for (quit == false) {
		
		inputVal = <- in;  // read a single input 
		fmt.Printf("----shuffle node (%d) got val %.3f \n",row,inputVal)
		out <- inputVal;  // copy to the two outputs 

		// poll the control channel 
		if (len(control) > 0) {
			msg = <- control;
			fmt.Printf("shuffle node %d got control message %d \n",row,msg)
			if (msg == 0xFF) {
				fmt.Printf("shuffle node %d ending \n",row,msg)
				return ; 
			}; 
		}; 
	};
}; 
	
// the input nodes copy one input to two output at the begining of the FFT
func input_node(row uint32, in chan complex128, out1 chan complex128, out2 chan complex128, control chan uint8 ) {
	var inputVal complex128;
	var msg uint8;
	var quit bool;

	quit = false;

	// while quit == false 	
	for (quit == false) {
		inputVal = <- in;  // read a single input 

		fmt.Printf("----input node (%d) got val %.3f \n",row,inputVal)
		
		out1 <- inputVal;  // copy to the two outputs 
		out2 <- inputVal;

		// poll the control channel 
		if (len(control) > 0) {
			msg = <- control;
			fmt.Printf("input node %d got control message %d \n",row,msg)
			if (msg == 0xFF) {
				fmt.Printf("input node %d ending \n",row,msg)
				return ; 
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

	quit = false;

	// while quit == false 	
	for (quit == false) {
		
		a = <- in1;       // read the inputs

		fmt.Printf(" == compute node (%d:%d) got A input %.2f \n",col,row,a)		
		b = <- in2;

		fmt.Printf(" == compute node (%d:%d) got B input %.2f \n",col,row,b)

	        value = a + Wn*b;  // this line is the main node computation

		fmt.Printf("----compute node (%d:%d) got inputs %.3f + %.3f * %.3f = %.3f \n",col,row,a,Wn,b,value)
		
		//out1 <- value;    // write the outputs 
		//out2 <- value;

		out1 <- complex(float64(col),float64(row))
		out2 <- complex(float64(col+100),float64(row+100))

		// poll the control channel 
		if (len(control) > 0) {
			msg = <- control;
			fmt.Printf("node %d:%d got control message %d \n",col,row,msg)
			if (msg == 0xFF) {
				fmt.Printf("node %d:%d ending \n",col,row,msg)
				return ; 
			}; 
		}; 
	}; 
} ;

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
	m = row % N         // need a bit-mask here, but use mod for now 

	// recall e^-i*2*Pi*m/N = cos(2*Pi*m/N) - i*sin(2*Pi*m/N)
	inner = 2.0*math.Pi*float64(m)/float64(N) ; // fraction on the unit circle to move 
	retval = complex(math.Cos(inner),-1.0*math.Sin(inner)) ; // definition

	fmt.Printf("twiddle for (%d,%d), m/N (%d,%d), twiddle %.3f \n", col,row,m,N,retval)
	
	
	return retval;
}

func create_fft_array(fft *FFTarray) {
	// make an array of channels 

	// each node has 2 inputs + 2 outputs, but outputs are shared as inputs
	// except at the edges, which are the length of a vector 
	const FFT_CHANNELS uint32 = (FFT_NODES *2) + (FFT_VSIZE*2)
	var r,c uint32; // the current column of row of the node to create
	var twiddle complex128

	
	// these are use to compute the target row for the b_channel channels in the butterfly
	var cross_distance_in, cross_distance_out uint32 ; // number of rows from current row
	var cross_bit_value_in, cross_bit_value_out uint32;  // direction 0=decreasing (up) 1=down
	var cross_row_input, cross_row_output uint32;  // the actual target row 

	// indexs to the channels in the main channel arrays
	var a_channel_in_id, b_channel_in_id, a_channel_out_id, b_channel_out_id uint32

	// pointers to the channel in the channel arrays 
	var a_channel_in, b_channel_in, a_channel_out, b_channel_out chan complex128

	
	fmt.Printf("fft sizes are %d %d %d \n", FFT_LOG, FFT_VSIZE, FFT_NODES) ; 

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
	// the correct set of channels. 
	for c = 0; c < FFT_LOG ; c++ {   // we have a log(fftsize) columns 
		for r = 0; r < FFT_VSIZE ; r++ {  // vector size rows 

			// get the distance from the current row to the target row for the cross input channel 
			cross_distance_in  = (1<<c)  // distance from current row for the input
			cross_distance_out = (1<<(c+1))  // the output channel is one column to the right, so the distance is larger

			// the value of the bit in the row number determines if the offset is up or down
			cross_bit_value_in = r & cross_distance_in
			cross_bit_value_out = r & cross_distance_out
			
			// target row for the cross input channel 
			if (cross_bit_value_in == 0) {
				cross_row_input = r +  cross_distance_in
				a_channel_in_id = r
				b_channel_in_id = cross_row_input

				a_channel_in = fft.a_channels[c][int(a_channel_in_id)]
				b_channel_in = fft.b_channels[c][int(b_channel_in_id)]

				fmt.Printf("Chan-Node (%d:%d) In: a is a_channel (%d:%d) b is b_channel (%d:%d) \n",c,r,c,a_channel_in_id,c,b_channel_in_id)
			} else {
				cross_row_input = r - cross_distance_in
				a_channel_in_id = cross_row_input
				b_channel_in_id = r

				a_channel_in = fft.b_channels[c][int(b_channel_in_id)]				
				b_channel_in = fft.a_channels[c][int(a_channel_in_id)]
				fmt.Printf("Chan-Node (%d:%d) In: a is cross (%d:%d) b is a_channel (%d:%d) \n",c,r,c,a_channel_in_id,c,b_channel_in_id)
			}


			// these are the output channel ID
			if (c < (FFT_LOG-1)) {  //output column 
				if (cross_bit_value_out == 0) {
					cross_row_output = r + cross_distance_out
					a_channel_out_id = r
					b_channel_out_id = cross_row_output

					a_channel_out = fft.a_channels[c+1][int(a_channel_out_id)]
					b_channel_out = fft.b_channels[c+1][int(b_channel_in_id)]

					fmt.Printf("Chan-Node (%d:%d) Out: a is a_channel (%d:%d) b is cross (%d:%d) \n",c,r,c+1,a_channel_out_id,c+1,b_channel_out_id)				
				
				} else {
					cross_row_output = r - cross_distance_out
					a_channel_out_id = cross_row_output 
					b_channel_out_id = r

					a_channel_out = fft.b_channels[c+1][int(a_channel_in_id)]
					b_channel_out = fft.a_channels[c+1][int(b_channel_out_id)]
				
					fmt.Printf("Chan-Node (%d:%d) Out: a is cross (%d:%d) b is a_channel (%d:%d) \n",c,r,c+1,a_channel_out_id,c+1,b_channel_out_id)				
				}
			} else{
				a_channel_out = fft.output_channels[int(r)]
				b_channel_out = nil
			}

			twiddle = compute_twiddle_factor(c,r)
			// we have to special case the input an output nodes
			if (c == 0) {
				fmt.Printf("Creating input node(%d) inputs: (%d) outputs: (%d:%d) (%d:%d) \n",r,r,c,r,c,cross_row_input)
				go input_node(r,fft.input_channels[r],fft.a_channels[c][r],fft.b_channels[c][r],fft.cntl_channels[FFT_LOG][r]);
			}
			if (c == (FFT_LOG-1)) {  //output column 
				go compute_node(c,r,a_channel_in,b_channel_in,fft.output_channels[r],nil,twiddle,fft.cntl_channels[c][r])

			} else { // interior columns/nodes
				go compute_node(c,r,a_channel_in,b_channel_in,a_channel_out,b_channel_out,twiddle,fft.cntl_channels[c][r])				
			}
		}
		fmt.Printf(" \n ");			
	}; // end for compute nodes
}

func read_outputs(fft *FFTarray) {
	var value complex128;

	for i, _ := range fft.output_channels {
		value = <- fft.output_channels[i] ;
		fmt.Printf("output: %d got val %.3f \n",i,value);		
	}


}

func main() {
	var fft *FFTarray;
	var signal[FFT_VSIZE] float64 ;
	var i, j,reversed int;
	var t float64 ;
	//var cv complex128; 
	//var r,c int;
	
	fft = new(FFTarray); 
	create_fft_array(fft);

	// test the FFT by sending in some signals and make sure we get the
	// right values in the frequency domain

	// Make a square wave 
	for i =0; int(i) < len(signal) ; i++  {

		//j = int(math.Floor(float64(i/2))) % 2
		//if (j == 0) { 
		//t = float64(1.0000);
		//} else {
		//t = float64(0.0000);
		//}
		j = (i); 
		t = float64(j);
		reversed = bitrev(i,int(FFT_LOG));
		fmt.Printf("signal[%d] was %d %d %0.3f \n",reversed,i,j,t);
		signal[reversed] = t;
	}

	/*
	i =0; 
	for c = 0; c < int(FFT_LOG) ; c++ {   // we have a log(fftsize) columns 
		for r = 0; r < int(FFT_VSIZE) ; r++ {  // vector size rows
			i++;
			cv = complex(float64(i),float64(0.0))
			fmt.Printf("sending value %.1f to a_channel_channel (%d:%d) \n",cv,c,r)
			fft.a_channels[c][r] <- cv 
			i++;
			cv = complex(float64(i),float64(0.0))
			fmt.Printf("sending value %.1f to cross_channel (%d:%d) \n",cv,c,r)
			fft.b_channels[c][r] <- cv

			fmt.Printf("sending ending value to (%d:%d) \n",c,r)
			fft.cntl_channels[c][r] <- uint8(0xFF)
		}
	}

	c= int(FFT_LOG+1)
	for r = 0; r < int(FFT_VSIZE) ; r++ {  // vector size rows
		fmt.Printf("sending ending value to (%d:%d) \n",c,r)
		fft.cntl_channels[c][r] <- uint8(0xFF)
	}
        */	

	
	for i =0; i < len(signal) ; i++  {
		fmt.Printf("signal[%d] is %.3f\n",i,signal[i]);		
		fft.input_channels[i] <- complex(signal[i], 0.0) ; 
	}
	
	time.Sleep(1*time.Second)
	if (1==1) {
		os.Exit(1);
	}
	read_outputs(fft);
}
