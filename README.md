# goxmlpath
# goxmlpath

## how to use it 
* go get github.com/zhangweilun/goxmlpath

example:
 `  
    ips := goxmlpath.MustCompile("//[@id=\"ip_list\"]/tbody/tr/td[2]")
  	page, err := goxmlpath.ParseHTML(res.Body)
  	defer res.Body.Close()
  	if err != nil {
  		log.Fatal(err)
  	}
  
  	items := ips.Iter(page)
  	for items.Next() {
  		nodes := items.Node()
  		fmt.Println(nodes.String())
  	}`
