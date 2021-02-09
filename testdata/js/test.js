var p = document.getElementById("color-changing-paragraph");

p.addEventListener("click", function() {
    var color = p.style.backgroundColor;
    if (color) {
        color = '';
    } else {
        color = '#99f';
    }
    p.style.backgroundColor = color;
});
