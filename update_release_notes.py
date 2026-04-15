with open('CHANGELOG.md', 'r', encoding='utf-8') as f:
    content = f.read()

start = content.find('## [v1.3.13]')
end = content.find('## [v1.3.12]')
if start != -1 and end != -1:
    notes = content[start:end].strip()
    with open('release_notes_v1.3.13.md', 'w', encoding='utf-8') as f:
        f.write(notes)
    print('done')
else:
    print('not found')
