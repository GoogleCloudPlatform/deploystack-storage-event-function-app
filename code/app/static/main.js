var basepath = "/api/v1/image";
var imagecount = 0;

document.addEventListener('DOMContentLoaded', function(){
    listImages();
    document.querySelector(".upload").addEventListener("click", uploadImage)
    document.querySelector("#myFile").addEventListener("change", activateUpload)
});

function activateUpload(){
    document.querySelector(".upload").disabled = false;
}

function sendAlert(message){
    let alert = document.querySelector(".alert");
    alert.innerHTML = message;
    alert.classList.remove("error");
    alert.style.display = "block";

    setTimeout(function(){
        console.log("should fade")
        document.querySelector(".alert").style.opacity = 0;
        setTimeout(function(){
            let alert = document.querySelector(".alert");
            alert.style.display = "none";
            alert.style.opacity = 1;

        }, 2000)
    }, 3000)
}

function sendError(message){
    let alert = document.querySelector(".alert");
    alert.classList.add("error");
    alert.innerHTML = message;
    alert.style.display = "block";

    setTimeout(function(){
        console.log("should fade")
        document.querySelector(".alert").style.opacity = 0;
        setTimeout(function(){
            let alert = document.querySelector(".alert");
            alert.style.opacity = 1;
            alert.style.display = "none";
            

        }, 2000)
    }, 3000)
}


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
    

    if (images.length == 0){
        content.innerHTML = "Your Gallery is empty. Upload Images to populate it!";
        return;
    }

    if (images.length == imagecount){
        setTimeout(listImages, 1000);
        return;
    }

    imagecount = images.length;

    content.innerHTML = "";

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

        let btn = document.createElement("button");
        btn.classList.add("delete-btn")
        btn.innerHTML = "<span class=\"material-icons\">delete</span>";
        btn.id = img.name;
        btn.addEventListener("click", deleteImage);
        div.appendChild(btn);

        content.appendChild(div);
    });

}

function deleteImage(e){
    console.log("delete" );
    let id = e.currentTarget.id

    var xmlhttp = new XMLHttpRequest();

    xmlhttp.onreadystatechange = function() {
        if (xmlhttp.readyState == XMLHttpRequest.DONE) {   // XMLHttpRequest.DONE == 4
           if (xmlhttp.status == 204) {
                sendAlert("Image Deleted.");  
                listImages();
           }
           else if (xmlhttp.status == 400) {
              alert('There was an error 400');
           }
           else {
               alert('something else other than 204 was returned');
               console.log(xmlhttp.status);
           }
        }
    };

    xmlhttp.open("DELETE", basepath+"/"+ id, true);
    xmlhttp.send();
}

function uploadImage(e){
    e.preventDefault();
    var xmlhttp = new XMLHttpRequest();
    let photo = document.getElementById("myFile").files[0];  // file from input
    let form  = new FormData();


    if (typeof photo == 'undefined'){
        alert('No image indicated');
        return;
    }
    
    form.append("myFile", photo);                                

    xmlhttp.onreadystatechange = function() {
        if (xmlhttp.readyState == XMLHttpRequest.DONE) {   // XMLHttpRequest.DONE == 4
           if (xmlhttp.status == 201) {
               // Waiting on Cloud Function to modify the images. 
               sendAlert("Processing image!");  
               setTimeout(listImages, 2000);
           }
           else if (xmlhttp.status == 500) {
            let msg = JSON.parse(xmlhttp.response);
            if (msg.error.includes("invalid image type")) {
                sendError(msg.error);
            } else {
                sendError(msg.error);
            }
           }
           else {
               alert('something else other than 201 was returned');
               console.log(xmlhttp.status);
           }
        }
    };

    xmlhttp.open("POST", basepath, true);
    xmlhttp.send(form);
}