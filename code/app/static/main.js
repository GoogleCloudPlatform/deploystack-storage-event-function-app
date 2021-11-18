var basepath = "/api/v1/image";

document.addEventListener('DOMContentLoaded', function(){
    listImages();
});



function listImages() {
    var xmlhttp = new XMLHttpRequest();

    xmlhttp.onreadystatechange = function() {
        if (xmlhttp.readyState == XMLHttpRequest.DONE) {   // XMLHttpRequest.DONE == 4
           if (xmlhttp.status == 200) {
               renderGallery(xmlhttp.response);
           }
           else if (xmlhttp.status == 400) {
              alert('There was an error 400');
           }
           else {
               alert('something else other than 200 was returned');
           }
        }
    };

    xmlhttp.open("GET", basepath, true);
    xmlhttp.send();
}

function renderGallery(resp){
    let images = JSON.parse(resp);
    let content = document.querySelector(".gallery");
    content.innerHTML = "";

    let ul = document.createElement("ul");
    ul.classList.add("list")

    images.forEach(img => {
        let div = document.createElement("div");
        div.classList.add("frame");
        let p = document.createElement("p");
        p.innerHTML = img.name;

        let a = document.createElement("a");
        a.href = img.original;

        let i = document.createElement("img");
        i.src = img.thumbnail;
        a.appendChild(i)
        a.appendChild(p)
        div.appendChild(a);




        content.appendChild(div);
    });

   


    

}