-- Add icon_url field to virtual_airlines table
ALTER TABLE virtual_airlines ADD COLUMN icon_url VARCHAR(512);

-- Default icon for existing VAs (Discord embed avatar)
UPDATE virtual_airlines
SET icon_url = 'https://cdn.discordapp.com/embed/avatars/0.png'
WHERE icon_url IS NULL;
