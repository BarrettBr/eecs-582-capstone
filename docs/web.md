# Web

## Site Structure
As of writing there are three views: Home, Dashboard, and About. These are all loaded into App.Vue, which mainly provides a dashboard for navigation. All components should have their style decalred in the same file for easy iteration.

## Dependencies and Justification
- **Vue**, used as the front end framework
- **Vite**, acts as a server for rapid front end development
- **PrimeVue**, This is a component library which allows for modern looking web design
- **Pinia**, lightweight and safe way to storing state
- **Charts.js**, a dependency for Vue charts allows for the generation of fancy data visualization

## Useful Folders & Files


| Path                         | Description                                                                 |
|------------------------------|-----------------------------------------------------------------------------|
| `/web/public`                | Stores public assets such as images and other static files.               |
| `/web/src/views`             | Holds the main "pages" for the front end.                                  |
| `/web/src/components`        | Contains reusable components that are combined to generate a view.         |
| `/web/src/App.vue`           | Defines the specifications for the single-page application and acts as the top-level component. |
| `/web/src/main.ts`           | Application entry point, initializes and mounts the app.                   |
| `/web/src/router/index.ts`   | Stores the url paths to specific views (/src/views)                        |
| `/web/src/assets/`           | used to store other static assets such as css files                        |


## Command quickstarts
- `npm install` will install all necessary dependencies, you will need to do this before running anything else
- `npm run dev` creates a dev server allowing you to view the files using vite
- `npx prettier --write .` will format all files in the current directory to the `.prettierrc` in `/web`
- `npm run build` will compile the vue into html/js etc in the `/web/dist/` directory


## Useful Links
- **Vue Docs**  
  https://vuejs.org/guide/quick-start.html

- **PrimeVue Docs**  
  https://primevue.org/setup/

- **Vue Router Docs**  
  https://router.vuejs.org/introduction.html

- **Pinia Docs**  
  https://pinia.vuejs.org/introduction.html

