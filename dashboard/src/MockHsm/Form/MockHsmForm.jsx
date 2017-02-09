import React from 'react'
import { BaseNew, FormContainer, FormSection, TextField } from 'features/shared/components'
import { reduxForm } from 'redux-form'
import mockHsmActions from 'MockHsm/mockHsmActions'

class MockHsmForm extends React.Component {
  constructor(props) {
    super(props)
  }

  render() {
    const {
      fields: { alias },
      error,
      handleSubmit,
      submitting
    } = this.props

    return(
      <FormContainer
        error={error}
        label='New MockHSM key'
        onSubmit={handleSubmit(this.props.createKey)}
        submitting={submitting} >

        <FormSection title='Key Information'>
          <TextField title='Alias' placeholder='Alias' fieldProps={alias} autoFocus={true} />
        </FormSection>
      </FormContainer>
    )
  }
}

const fields = [ 'alias' ]
export default BaseNew.connect(
  BaseNew.mapStateToProps('mockhsm'),
  (dispatch) => ({
    createKey: body => dispatch(mockHsmActions.createKey(body))
  }),
  reduxForm({
    form: 'newMockHsmKey',
    fields,
  })(MockHsmForm)
)
