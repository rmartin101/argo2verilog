// small program to test for statements


package main ;

import ( "fmt" ) ;

func snafu(i,j int) int {
	var a int ;
	a = i;
	if (i <= j) {
		return i*j*a ;
	} ;

	return i*j - (i-j) ;
} ;

func plusOne(i int) int {
	for i < 3 {
		i = i +1; 
	};

	return i + 1; 
};

func main() {
	var i,j,k int ; 
	
	i = 1 ;
	j = 2 ; 
	k = 4 ;

	sum := 0x0000; 
	for i := 1; i < k ; i = i + 1 {
		sum = sum + i;
		j = snafu(j,j);
	}; 
	fmt.Printf("The sum is %d \n",sum) ;

	j =  1;
	for j < 5 {
		j = j*2;
	};
	
	fmt.Printf("j is %d\n",j) ; 

	sum =0 ;
	for {
		sum = sum +1;
		if (sum > 4) {
			break;
		}; 
		fmt.Printf("Inside the sum is %d \n",sum) ;	
	};
	fmt.Printf("Outer sum is %d \n",sum) ;
	sum = 0;
	for i := plusOne(i); i < 7 ; i = i + 1 {
		sum = sum + i;
		if (sum >3) {
			continue;
		};
		fmt.Printf("Looped sum is %d \n",sum) ;		
	};
	
	fmt.Printf("Final sum is %d \n",sum) ;
	fmt.Printf("End of Program \n") ;	
} ;
