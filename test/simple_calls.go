// very simple IF statement test 

package main ;

import ( "fmt" ) ;

func blammo(input int) int {
	var i int;

	i = input;
	
	return i+i;
};

func decoid(input,output int) int {
	output = input;
	if (input > 3) {
		return output + input;
	} else {
		return input + input + output ;
	}; 
};

func main() {
	var i,j,k,z int;
	
	// simple if statement 
	if  (i < j) {
		fmt.Printf("I is less than J \n") ; 
	} ;

	fmt.Printf("I j k are %d %d %d \n", i,j,k ) ;
	
	// if with an else 
	if  (k >= (i + 3)) {
		i = 4;
		k = i + i;
		j = blammo(i);
		fmt.Printf("K is: %d \n", k) ;
	} else {
		fmt.Printf("I and J are:: %d %d  \n", k,j) ;
	} ;

	i = blammo(j);
	
	// a chained if-else 
	z = i+j+k+20 ;
	fmt.Printf("Z is %d  \n", z) ;
	if   ( (j+k) > z )  {
		z = 0;
		fmt.Printf(" Z is %d  \n", z) ; 
	} else if ( (z+z) > 3000) {
		fmt.Printf("z+z is %d  \n", z+z) ;
	} else {
		fmt.Printf("End of the chained if \n") ;
	} ;

	k = i+j;
	fmt.Printf("K is %d \n",k) ;
} ;

