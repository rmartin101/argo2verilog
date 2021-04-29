// very simple IF statement test 

package main ;

import ( "fmt" ) ;


func main() {
	var i,j int16 ; 

	i = 7 ;
	j = 5 ; 

	// simple if statement 
	if  (i > j) {
		fmt.Printf("I is greater than J \n") ; 
	 }  else {
		 fmt.Printf("I is less than J  \n") ; 
	} ; 

	fmt.Printf("before loop,i is %d j is %d \n",i,j) ;
	// simple for statement 
	for i = 0; i < j ; i = i + 1  {
		for j = 0 ; j < 4; j = j + 1 {
			fmt.Printf("inner loop, i is %d and j is %d\n",i,j) ;
		};
	} ;

	fmt.Printf("outer loop,i is %d j is %d \n",i,j) ;	

} ;

