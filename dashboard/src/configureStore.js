/*eslint-env node*/

import { createStore, applyMiddleware, compose } from 'redux'
import thunkMiddleware from 'redux-thunk'
import { routerMiddleware as createRouterMiddleware } from 'react-router-redux'
import createSagaMiddleware from 'redux-saga'
import { history } from 'utility/environment'
import { exportState, importState } from 'utility/localStorage'

import makeRootReducer from 'reducers'
import sagas from 'sagas'

const routerMiddleware = createRouterMiddleware(history)
const sagaMiddleware = createSagaMiddleware()

export default function() {
  const store = createStore(
    makeRootReducer(),
    importState(),
    compose(
      applyMiddleware(
        thunkMiddleware,
        routerMiddleware,
        sagaMiddleware
      ),
      window.devToolsExtension ? window.devToolsExtension() : f => f
    )
  )

  // Enable sagas
  sagaMiddleware.run(sagas)

  if (module.hot) {
    // Enable Webpack hot module replacement for reducers
    module.hot.accept('reducers', () => {
      const newRootReducer = require('reducers').default
      store.replaceReducer(newRootReducer())
    })
  }

  store.subscribe(exportState(store))

  return store
}
