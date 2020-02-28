// Small program to test popelined channels
// the second stage demonstrates a tiny filter

package main ;

import ( "fmt" ) ;

func input(total uint32, pipe1 chan uint32) {
	var i uint32; 

	pipe1 <- total ;  // end flag value 	
	for i = 0; i < total; i = i + 1  {
		pipe1 <- i ;
		fmt.Printf("input: count %d sent integer %d\n",i,i);
	}; 

}; 

func filter(pipe1 chan uint32, pipe2 chan uint32){
	var count,total,val uint32 ;
	
	count = 0;
	total = <- pipe1;

	pipe2 <- total ;
	// send the rest of the message 
	for (count < total) {
		val = <- pipe1 ;
		if ((val & 0x00000007) == 3) || ((val & 0x00000007)== 4) { // very simple filter here 
			val = val | 0x00000007;
		};
		pipe2 <- val ; 
		count = count + 1; 
	}; 
}; 

func output(pipe2 chan uint32,done chan bool) {
	var count,total,val uint32;

	count = 0;
	total = <- pipe2;	
	for (count < total) {
		val =	<- pipe2 ;
		fmt.Printf("output: count: %d got integer %d\n",count,val);
		count = count +1;
	};

	done <- true; 
};
	

func main() {
	var finished bool;
	
	pipe1 := make(chan uint32,1) ;
	pipe2 := make(chan uint32, 4);
	done := make(chan bool, 1);
	
	go output(pipe2,done) ;
	go filter(pipe1,pipe2) ;
	go input(10,pipe1) ;

	finished = <- done;

	fmt.Printf("finished is %t\n", finished);
	
};

