const form = document.getElementById('note-form');
const input = document.getElementById('note-input');
const list = document.getElementById('note-list');

function loadNotes() {
    const notes = JSON.parse(localStorage.getItem('notes') || '[]');
    list.innerHTML = '';
    notes.forEach(note => {
        const li = document.createElement('li');
        li.textContent = note;
        list.appendChild(li);
    });
}

form.addEventListener('submit', (e) => {
    e.preventDefault();
    const notes = JSON.parse(localStorage.getItem('notes') || '[]');
    notes.push(input.value);
    localStorage.setItem('notes', JSON.stringify(notes));
    input.value = '';
    loadNotes();
});

loadNotes();
