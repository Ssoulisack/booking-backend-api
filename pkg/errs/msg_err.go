package errs

const ErrMsgPaymentMethodNotFound string = "PAYMENT_METHOD_NOT_FOUND"
const ErrMsgCampaignNotFound string = "CAMPAIGN_NOT_FOUND"
const ErrMsgAnErrorOccurredWithStripeAPI string = "AN_ERROR_OCCURRED_WITH_STRIPE_API"
const ErrMsgTheRequestWasMalformedOrMissingARequiredParameter string = "THE_REQUEST_WAS_MALFORMED_OR_MISSING_A_REQUIRED_PARAMETER"
const ErrMsgTheRequestedResourceDoesNotExist string = "THE_REQUESTED_RESOURCE_DOES_NOT_EXIST"
const ErrMsgAnErrorOccurredDueToRequestsHittingTheAPITooQuickly string = "AN_ERROR_OCCURRED_DUE_TO_REQUESTS_HITTING_THE_API_TOO_QUICKLY"
const ErrMsgThereIsNoCardOnACustomerThatIsBeingCharged string = "THERE_IS_NO_CARD_ON_A_CUSTOMER_THAT_IS_BEING_CHARGED"
const ErrMsgCustomerAlreadyExist string = "CUSTOMER_ALREADY_EXIST"
const ErrMsgYourCardZipCodeFailedValidation string = "YOUR_CARD_ZIPCODE_FAILED_VALIDATION"
const ErrMsgYourCardSwipeDataIsInvalid string = "YOUR_CARD_SWIPE_DATA_IS_INVALID"
const ErrMsgYourCardCVCIsInvalid string = "YOUR_CARD_CVC_IS_INVALID"
const ErrMsgYourCardExpiryYearIsInvalid string = "YOUR_CARD_EXPIRY_YEAR_IS_INVALID"
const ErrMsgYourCardExpiryMonthIsInvalid string = "YOUR_CARD_EXPIRY_MONTH_IS_IN_VALID"
const ErrMsgYourCardNumberIsIncorrect string = "YOUR_CARD_NUMBER_IS_IN_CORRECT"
const ErrMsgAnErrorOccurredWhileProcessingYourCard string = "AN_ERROR_OCCURRED_WHILE_PROCESSING_YOUR_CARD"
const ErrMsgYourCardHasExpired string = "YOUR_CARD_HAS_EXPIRED"
const ErrMsgYourCardCVCIsIncorrect string = "YOUR_CARD_CVC_IS_INCORRECT"
const ErrMsgYourCardWasDeclined string = "YOUR_CARD_WAS_DECLINED"
const ErrMsgBadRequest string = "BAD_REQUEST"
const ErrMsgInvalidAccessToken string = "INVALID_ACCESS_TOKEN"
const ErrMsgOtpExpired string = "OTP_EXPIRED"
const ErrMsgInvalidOtp string = "INVALID_OTP"
const ErrMsgOtpAlreadyVerified string = "OTP_ALREADY_VERIFIED"
const ErrMsgParamIdIsRequired string = "PARAM_ID_IS_REQUIRED"
const ErrMsgInternalServerError string = "INTERNAL_SERVER_ERROR"
const ErrMsgUnauthorized string = "UNAUTHORIZED"
const YourRoleNotAllowedToAccessThisResource string = "YOUR_ROLE_NOT_ALLOWED_TO_ACCESS_THIS_RESOURCE"
const YourPermissionNotAllowedToAccessThisResource string = "YOUR_PERMISSION_NOT_ALLOWED_TO_ACCESS_THIS_RESOURCE"
const ErrMsgInvalidToken string = "INVALID_TOKEN"
const ErrMsgUserNotFound string = "USER_NOT_FOUND"
const ErrMsgUserDisable string = "USER_DISABLE"
const ErrMsgUserOrPasswordNotMatch string = "USER_OR_PASSWORD_NOT_MATCH"
const ErrMsgUserAlreadyExist string = "USER_ALREADY_EXIST"
const ErrMsgNewPasswordAndConfirmPasswordNotMatch string = "NEW_PASSWORD_AND_CONFIRM_PASSWORD_NOT_MATCH"
const ErrMsgDuplicateRoleName string = "DUPLICATE_ROLE_NAME"
const ErrMsgDuplicatePermissionName string = "DUPLICATE_PERMISSION_NAME"
const InvalidDatetimeFormat string = "INVALID_DATETIME_FORMAT_EX:(yyyy-mm-dd hh:mm:ss)"
const InvalidDateFormat string = "INVALID_DATETIME_FORMAT_EX:(yyyy-mm-dd)"
const ErrMsgMoneyIsRequired string = "MONEY_IS_REQUIRED"
const ErrMsgDigitIsRequired string = "DIGIT_IS_REQUIRED"
const ErrMsgDigitIsInvalid string = "DIGIT_IS_INVALID"
const ErrMsgMoneyIsInvalid string = "MONEY_IS_INVALID"
const ErrMsgQuotaIsNotEnough string = "QUOTA_IS_NOT_ENOUGH"
const YouAreNotOwnerOfThisBill string = "YOU_ARE_NOT_OWNER_OF_THIS_BILL"
const ErrMsgDigitIsDuplicate string = "DIGIT_IS_DUPLICATE"

const ErrMsgDataNotFound string = "DATA_NOT_FOUND"
const RouteNotFound string = "ROUTE_NOT_FOUND"

// ErrRecordNotFound record not found error
const ErrRecordNotFound = "RECORD_NOT_FOUND"

// ErrInvalidTransaction invalid transaction when you are trying to `Commit` or `Rollback`
const ErrInvalidTransaction = "INVALID_TRANSACTION"

// ErrNotImplemented not implemented
const ErrNotImplemented = "NOT_IMPLEMENTED"

// ErrMissingWhereClause missing where clause
const ErrMissingWhereClause = "WHERE_CONDITIONS_REQUIRED"

// ErrUnsupportedRelation unsupported relations
const ErrUnsupportedRelation = "UNSUPPORTED_RELATIONS"

// ErrPrimaryKeyRequired primary keys required
const ErrPrimaryKeyRequired = "PRIMARY KEY REQUIRED"

// ErrModelValueRequired model value required
const ErrModelValueRequired = "MODEL_VALUE_REQUIRED"

// ErrModelAccessibleFieldsRequired model accessible fields required
const ErrModelAccessibleFieldsRequired = "MODEL_ACCESSIBLE_FIELDS_REQUIRED"

// ErrSubQueryRequired sub erp_query_repo required
const ErrSubQueryRequired = "SUB_QUERY_REQUIRED"

// ErrInvalidData unsupported data
const ErrInvalidData = "UNSUPPORTED_DATA"

// ErrUnsupportedDriver unsupported driver
const ErrUnsupportedDriver = "UNSUPPORTED_DRIVER"

// ErrRegistered registered
const ErrRegistered = "REGISTERED"

// ErrInvalidField invalid field
const ErrInvalidField = "INVALID_FIELD"

// ErrEmptySlice empty slice found
const ErrEmptySlice = "EMPTY_SLICE_FOUND"

// ErrDryRunModeUnsupported dry run mode unsupported
const ErrDryRunModeUnsupported = "DRY_RUN_MODE_UNSUPPORTED"

// ErrInvalidDB invalid db
const ErrInvalidDB = "INVALID_DB"

// ErrInvalidValue invalid value
const ErrInvalidValue = "INVALID_VALUE_SHOULD_BE_POINTER_TO_STRUCT_OR_SLICE"

// ErrInvalidValueOfLength invalid values do not match length
const ErrInvalidValueOfLength = "INVALID_ASSOCIATION_VALUES,_LENGTH_DO_NOT_MATCH"

// ErrPreloadNotAllowed preload is not allowed when count is used
const ErrPreloadNotAllowed = "PRELOAD_IS_NOT_ALLOWED_WHEN_COUNT_IS_USED"

// ErrDuplicatedKey occurs when there is a unique key constraint violation
const ErrDuplicatedKey = "DUPLICATED_KEY_NOT_ALLOWED"

const ErrPackageCodeNotMatch = "PACKAGE_CODE_NOT_MATCH"
const ErrPackageHasBeenUsed = "PACKAGE_HAS_BEEN_USED"
const ErrPackageHasBeenCancelled = "PACKAGE_HAS_BEEN_CANCELLED"
const ErrInternalServerError = "INTERNAL_SERVER_ERROR"

const ErrStatusTooManyRequests = "TOO_MANY_REQUESTS"

const ErrDataAlreadyDeleted = "DATA_ALREADY_DELETED"

const ErrMethodNotMatch = "ERR_METHOD_NOT_MATCH"
const InvalidUsernameOrPassword = "INVALID_USERNAME_OR_PASSWORD"
const ErrMsgPasswordNotMatch = "PASSWORD_NOT_MATCH"
